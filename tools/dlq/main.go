// Command dlq lists messages parked on <topic>.dlq topics and replays them to their
// source topics.
//
//	dlq list   [--topic T]              every parked message, pending or replayed
//	dlq replay [--topic T] [--dry-run]  republish pending messages, one per original event
//	dlq skip   [--topic T]              mark pending messages as handled without replaying
//
// --topic limits a command to the DLQ of one source topic, e.g. like.created.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/segmentio/kafka-go"
	"github.com/vahan-sahakyan/distributed-social-network/pkg/broker"
)

// group records how far each DLQ partition has been replayed
const group = "dlq-replay"

func main() {
	log.SetFlags(0)
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "localhost:19092"
	}
	if len(os.Args) < 2 {
		log.Fatal("usage: dlq list|replay|skip [--topic T] [--dry-run]")
	}
	fs := flag.NewFlagSet(os.Args[1], flag.ExitOnError)
	topic := fs.String("topic", "", "only the DLQ of this source topic, e.g. like.created")
	dryRun := fs.Bool("dry-run", false, "replay: show what would be replayed")
	_ = fs.Parse(os.Args[2:])

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	k := &kafkaDLQ{brokers: strings.Split(brokers, ","), client: &kafka.Client{Addr: kafka.TCP(strings.Split(brokers, ",")...)}, topic: *topic}

	var err error
	switch os.Args[1] {
	case "list":
		var letters []parked
		if letters, _, err = k.load(ctx); err == nil {
			printList(letters)
		}
	case "replay":
		err = k.replay(ctx, *dryRun)
	case "skip":
		err = k.skip(ctx)
	default:
		err = fmt.Errorf("unknown command %q: want list, replay or skip", os.Args[1])
	}
	if err != nil {
		log.Fatal(err)
	}
}

type kafkaDLQ struct {
	brokers []string
	client  *kafka.Client
	// topic, when set, limits everything to its DLQ
	topic string
}

type partition struct {
	topic     string
	id        int
	committed int64
	high      int64
}

// load reads every DLQ partition up to its high watermark.
func (k *kafkaDLQ) load(ctx context.Context) ([]parked, []partition, error) {
	parts, err := k.partitions(ctx)
	if err != nil {
		return nil, nil, err
	}
	var letters []parked
	for _, p := range parts {
		msgs, err := k.read(ctx, p)
		if err != nil {
			return nil, nil, fmt.Errorf("reading %s/%d: %w", p.topic, p.id, err)
		}
		for _, m := range msgs {
			l, err := parse(m, p.committed)
			if err != nil {
				log.Printf("skipping %s/%d@%d: %v", m.Topic, m.Partition, m.Offset, err)
				continue
			}
			letters = append(letters, l)
		}
	}
	sort.SliceStable(letters, func(i, j int) bool { return letters[i].Letter.FailedAt.Before(letters[j].Letter.FailedAt) })
	return letters, parts, nil
}

func (k *kafkaDLQ) partitions(ctx context.Context) ([]partition, error) {
	meta, err := k.client.Metadata(ctx, &kafka.MetadataRequest{})
	if err != nil {
		return nil, err
	}
	var parts []partition
	offsets := map[string][]kafka.OffsetRequest{}
	for _, t := range meta.Topics {
		if !strings.HasSuffix(t.Name, ".dlq") || (k.topic != "" && t.Name != k.topic+".dlq") {
			continue
		}
		for _, p := range t.Partitions {
			parts = append(parts, partition{topic: t.Name, id: p.ID})
			offsets[t.Name] = append(offsets[t.Name], kafka.LastOffsetOf(p.ID))
		}
	}
	if len(parts) == 0 {
		return nil, nil
	}

	last, err := k.client.ListOffsets(ctx, &kafka.ListOffsetsRequest{Topics: offsets})
	if err != nil {
		return nil, err
	}
	fetch := map[string][]int{}
	for _, p := range parts {
		fetch[p.topic] = append(fetch[p.topic], p.id)
	}
	committed, err := k.client.OffsetFetch(ctx, &kafka.OffsetFetchRequest{GroupID: group, Topics: fetch})
	if err != nil {
		return nil, err
	}
	for i, p := range parts {
		for _, o := range last.Topics[p.topic] {
			if o.Partition == p.id {
				parts[i].high = o.LastOffset
			}
		}
		for _, o := range committed.Topics[p.topic] {
			if o.Partition == p.id && o.CommittedOffset > 0 {
				parts[i].committed = o.CommittedOffset
			}
		}
	}
	return parts, nil
}

func (k *kafkaDLQ) read(ctx context.Context, p partition) ([]kafka.Message, error) {
	if p.high == 0 {
		return nil, nil
	}
	r := kafka.NewReader(kafka.ReaderConfig{Brokers: k.brokers, Topic: p.topic, Partition: p.id, MaxWait: time.Second})
	defer func() { _ = r.Close() }()
	if err := r.SetOffset(kafka.FirstOffset); err != nil {
		return nil, err
	}
	var msgs []kafka.Message
	for {
		m, err := r.ReadMessage(ctx)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, m)
		if m.Offset >= p.high-1 {
			return msgs, nil
		}
	}
}

func (k *kafkaDLQ) replay(ctx context.Context, dryRun bool) error {
	letters, parts, err := k.load(ctx)
	if err != nil {
		return err
	}
	msgs := plan(letters)
	pending := 0
	for _, l := range letters {
		if l.Pending {
			pending++
		}
	}
	fmt.Printf("%d pending parked messages -> %d events to replay\n", pending, len(msgs))
	for _, m := range msgs {
		id := m.ID
		if id == "" {
			id = "replay-of " + m.ReplayOf
		}
		fmt.Printf("  %s key=%s (%s)\n", m.Topic, m.Key, id)
	}
	if dryRun || len(msgs) == 0 {
		return nil
	}

	producer := broker.NewProducer(strings.Join(k.brokers, ","))
	defer producer.Close()
	if err := producer.PublishMessages(ctx, msgs); err != nil {
		return fmt.Errorf("replaying: %w", err)
	}

	if err := k.markDone(ctx, parts); err != nil {
		return fmt.Errorf("replayed, but recording progress failed (a rerun replays again): %w", err)
	}
	fmt.Printf("replayed %d events\n", len(msgs))
	return nil
}

func (k *kafkaDLQ) skip(ctx context.Context) error {
	letters, parts, err := k.load(ctx)
	if err != nil {
		return err
	}
	pending := 0
	for _, l := range letters {
		if l.Pending {
			pending++
		}
	}
	if err := k.markDone(ctx, parts); err != nil {
		return err
	}
	fmt.Printf("skipped %d pending parked messages\n", pending)
	return nil
}

// markDone records every loaded message as handled, so the next replay starts after them.
func (k *kafkaDLQ) markDone(ctx context.Context, parts []partition) error {
	commits := map[string][]kafka.OffsetCommit{}
	for _, p := range parts {
		if p.high > p.committed {
			commits[p.topic] = append(commits[p.topic], kafka.OffsetCommit{Partition: p.id, Offset: p.high})
		}
	}
	if len(commits) == 0 {
		return nil
	}
	resp, err := k.client.OffsetCommit(ctx, &kafka.OffsetCommitRequest{GroupID: group, GenerationID: -1, Topics: commits})
	if err != nil {
		return err
	}
	for topic, ps := range resp.Topics {
		for _, p := range ps {
			if p.Error != nil {
				return fmt.Errorf("%s/%d: %w", topic, p.Partition, p.Error)
			}
		}
	}
	return nil
}

func printList(letters []parked) {
	if len(letters) == 0 {
		fmt.Println("no parked messages")
		return
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	_, _ = fmt.Fprintln(w, "STATE\tAGE\tGROUP\tORIGINAL\tKEY\tERROR")
	for _, l := range letters {
		state := "replayed"
		if l.Pending {
			state = "pending"
		}
		_, _ = fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", state, age(l.Letter.FailedAt), orDash(l.Letter.Group),
			l.Letter.Origin(), l.Letter.Key, truncate(l.Letter.Error, 70))
	}
	_ = w.Flush()
}

func age(t time.Time) string {
	d := time.Since(t).Round(time.Second)
	switch {
	case d < time.Minute:
		return d.String()
	case d < time.Hour:
		return d.Round(time.Minute).String()
	default:
		return d.Round(time.Hour).String()
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
