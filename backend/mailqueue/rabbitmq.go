package mailqueue

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
)

func RunDispatcher(ctx context.Context, store SQLStore, brokerURL string, pollDelay time.Duration) error {
	connection, err := amqp.Dial(brokerURL)
	if err != nil {
		return fmt.Errorf("connect outbox dispatcher to RabbitMQ: %w", err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open outbox dispatcher channel: %w", err)
	}
	defer channel.Close()
	if err := declareQueues(channel); err != nil {
		return err
	}
	confirms := make(chan amqp.Confirmation, 1)
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("enable RabbitMQ publisher confirms: %w", err)
	}
	channel.NotifyPublish(confirms)

	for {
		published, err := store.PublishNext(ctx, func(ctx context.Context, id int64) error {
			body, err := json.Marshal(Event{ID: id})
			if err != nil {
				return fmt.Errorf("encode email event: %w", err)
			}
			return publishConfirmed(ctx, channel, confirms, QueueName, body, nil)
		})
		if err != nil {
			return err
		}
		if !published {
			if err := Wait(ctx, pollDelay); err != nil {
				return nil
			}
		}
	}
}

func RunWorker(ctx context.Context, store SQLStore, brokerURL string, sender Sender) error {
	connection, err := amqp.Dial(brokerURL)
	if err != nil {
		return fmt.Errorf("connect notification worker to RabbitMQ: %w", err)
	}
	defer connection.Close()

	channel, err := connection.Channel()
	if err != nil {
		return fmt.Errorf("open notification worker channel: %w", err)
	}
	defer channel.Close()
	if err := declareQueues(channel); err != nil {
		return err
	}
	if err := channel.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set notification worker prefetch: %w", err)
	}
	confirms := make(chan amqp.Confirmation, 1)
	if err := channel.Confirm(false); err != nil {
		return fmt.Errorf("enable dead-letter publisher confirms: %w", err)
	}
	channel.NotifyPublish(confirms)

	deliveries, err := channel.Consume(
		QueueName, "", false, false, false, false, nil,
	)
	if err != nil {
		return fmt.Errorf("consume email queue: %w", err)
	}
	handler := Handler{Store: store, Sender: sender}
	for {
		select {
		case <-ctx.Done():
			return nil
		case delivery, ok := <-deliveries:
			if !ok {
				return fmt.Errorf("RabbitMQ closed the email consumer")
			}
			handleDelivery(ctx, channel, confirms, handler, store, delivery)
		}
	}
}

func handleDelivery(
	ctx context.Context,
	channel *amqp.Channel,
	confirms <-chan amqp.Confirmation,
	handler Handler,
	store SQLStore,
	delivery amqp.Delivery,
) {
	var event Event
	if err := json.Unmarshal(delivery.Body, &event); err != nil || event.ID <= 0 {
		if err := publishConfirmed(ctx, channel, confirms, DeadQueueName, delivery.Body, nil); err != nil {
			logDeliveryError("publish malformed event to dead-letter queue", err)
			logDeliveryError("requeue malformed event", delivery.Nack(false, true))
			return
		}
		logDeliveryError("ack malformed event", delivery.Ack(false))
		return
	}

	err := handler.Handle(ctx, event.ID)
	if err == nil {
		logDeliveryError("ack processed event", delivery.Ack(false))
		return
	}
	logDeliveryError("process email event; scheduling retry", err)
	retryCount := RetryCount(delivery.Headers)
	if retryCount+1 < MaxDeliveryRuns {
		headers := amqp.Table{"x-shorty-retry": int32(retryCount + 1)}
		if publishErr := publishConfirmed(
			ctx, channel, confirms, retryQueueName(retryCount), delivery.Body, headers,
		); publishErr != nil {
			logDeliveryError("publish retry event", publishErr)
			logDeliveryError("requeue email event", delivery.Nack(false, true))
			return
		}
		logDeliveryError("ack scheduled retry", delivery.Ack(false))
		return
	}

	if err := publishConfirmed(ctx, channel, confirms, DeadQueueName, delivery.Body, nil); err != nil {
		logDeliveryError("publish event to dead-letter queue", err)
		logDeliveryError("requeue email event", delivery.Nack(false, true))
		return
	}
	if err := store.MarkFailed(ctx, event.ID, err.Error()); err != nil {
		log.Printf("email queue could not mark event %d failed: %v", event.ID, err)
		logDeliveryError("ack dead-lettered event", delivery.Ack(false))
		return
	}
	logDeliveryError("ack dead-lettered event", delivery.Ack(false))
}

func publishConfirmed(
	ctx context.Context,
	channel *amqp.Channel,
	confirms <-chan amqp.Confirmation,
	queue string,
	body []byte,
	headers amqp.Table,
) error {
	if err := channel.PublishWithContext(ctx, "", queue, false, false, amqp.Publishing{
		ContentType:  "application/json",
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		Body:         body,
		Headers:      headers,
	}); err != nil {
		return fmt.Errorf("publish RabbitMQ event to %s: %w", queue, err)
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case confirmation := <-confirms:
		if !confirmation.Ack {
			return fmt.Errorf("RabbitMQ rejected event for %s", queue)
		}
		return nil
	}
}

func declareQueues(channel *amqp.Channel) error {
	_, err := channel.QueueDeclare(QueueName, true, false, false, false, amqp.Table{})
	if err != nil {
		return fmt.Errorf("declare email queue: %w", err)
	}
	for attempt, delay := range retryDelays {
		_, err := channel.QueueDeclare(retryQueueName(int64(attempt)), true, false, false, false, amqp.Table{
			"x-message-ttl":             int32(delay / time.Millisecond),
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": QueueName,
		})
		if err != nil {
			return fmt.Errorf("declare email retry queue %d: %w", attempt+1, err)
		}
	}
	if _, err := channel.QueueDeclare(DeadQueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("declare email dead-letter queue: %w", err)
	}
	return nil
}
