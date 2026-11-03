package weblog_test

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/rshade/finfocus/internal/weblog"
)

func mk(source, kind string) weblog.Entry {
	return weblog.Entry{Level: "info", Source: source, SourceKind: kind, Message: "m"}
}

func TestHubSequenceStartsAtOne(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	for want := uint64(1); want <= 3; want++ {
		assert.Equal(t, want, h.Append(mk("core", weblog.KindCore)).Seq)
	}
}

func TestHubDefaultCapacity(t *testing.T) {
	t.Parallel()
	assert.Equal(t, 5000, weblog.NewHub(0).Status().Capacity)
	assert.Equal(t, 5000, weblog.NewHub(-3).Status().Capacity)
}

func TestHubEvictsOldest(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(3)
	for range 5 {
		h.Append(mk("core", weblog.KindCore))
	}
	st := h.Status()
	assert.True(t, st.Enabled)
	assert.Equal(t, 3, st.Capacity)
	assert.Equal(t, 3, st.Retained)
	assert.Equal(t, uint64(2), st.Dropped)

	snap, _, cancel := h.Subscribe(0)
	defer cancel()
	require.Len(t, snap.Entries, 3)
	assert.Equal(t, uint64(3), snap.Entries[0].Seq)
	assert.Equal(t, uint64(5), snap.Entries[2].Seq)
	assert.Equal(t, uint64(2), snap.Missed)
	assert.Equal(t, uint64(2), snap.Dropped)
	assert.Equal(t, 3, snap.Capacity)
}

func TestHubSubscribeAfterSeq(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		after      uint64
		wantFirst  uint64
		wantCount  int
		wantMissed uint64
	}{
		{"from start", 0, 1, 5, 0},
		{"middle", 2, 3, 3, 0},
		{"last", 5, 0, 0, 0},
		{"beyond", 99, 0, 0, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			h := weblog.NewHub(10)
			for range 5 {
				h.Append(mk("core", weblog.KindCore))
			}
			snap, _, cancel := h.Subscribe(tt.after)
			defer cancel()
			require.Len(t, snap.Entries, tt.wantCount)
			if tt.wantCount > 0 {
				assert.Equal(t, tt.wantFirst, snap.Entries[0].Seq)
			}
			assert.Equal(t, tt.wantMissed, snap.Missed)
		})
	}
}

func TestHubSubscribeMissedWhenResumingPastEviction(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(2)
	for range 6 {
		h.Append(mk("core", weblog.KindCore))
	}
	snap, _, cancel := h.Subscribe(1)
	defer cancel()
	assert.Equal(t, uint64(3), snap.Missed)
	require.Len(t, snap.Entries, 2)

	snap2, _, cancel2 := h.Subscribe(4)
	defer cancel2()
	assert.Zero(t, snap2.Missed)
	assert.Len(t, snap2.Entries, 2)
}

func TestHubSourcesRegisteredOnce(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	h.Append(mk("core", weblog.KindCore))
	h.Append(mk("aws", weblog.KindPlugin))
	h.Append(mk("core", weblog.KindCore))
	h.Append(mk("aws", weblog.KindPlugin))
	want := []weblog.Source{{Name: "core", Kind: "core"}, {Name: "aws", Kind: "plugin"}}
	assert.Equal(t, want, h.Status().Sources)
	snap, _, cancel := h.Subscribe(0)
	defer cancel()
	assert.Equal(t, want, snap.Sources)
}

func TestHubSourcesSurviveEviction(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(1)
	h.Append(mk("aws", weblog.KindPlugin))
	h.Append(mk("core", weblog.KindCore))
	assert.Len(t, h.Status().Sources, 2)
}

func TestHubLiveDelivery(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	_, ch, cancel := h.Subscribe(0)
	defer cancel()
	h.Append(mk("core", weblog.KindCore))
	got := <-ch
	assert.Equal(t, uint64(1), got.Seq)
}

func TestHubCancelIsIdempotentAndClosesChannel(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(10)
	_, ch, cancel := h.Subscribe(0)
	cancel()
	cancel()
	_, open := <-ch
	assert.False(t, open)
	h.Append(mk("core", weblog.KindCore))
}

func TestHubLaggingSubscriberIsClosedWithoutBlocking(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(1000)
	_, lagging, cancelLag := h.Subscribe(0)
	defer cancelLag()
	_, healthy, cancelOK := h.Subscribe(0)
	defer cancelOK()

	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 300 {
			h.Append(mk("core", weblog.KindCore))
			select {
			case <-healthy:
			default:
			}
		}
	}()
	<-done

	received := 0
	for range lagging {
		received++
	}
	assert.Equal(t, 256, received)

	h.Append(mk("core", weblog.KindCore))
	cancelLag()
}

func TestHubConcurrentAppendAndSubscribe(t *testing.T) {
	t.Parallel()
	h := weblog.NewHub(100)
	const writers, perWriter = 8, 500

	var wg sync.WaitGroup
	for w := range writers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			name := string(rune('a' + w))
			for range perWriter {
				h.Append(mk(name, weblog.KindPlugin))
			}
		}()
	}
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for range 50 {
				snap, ch, cancel := h.Subscribe(0)
				var prev uint64
				for _, e := range snap.Entries {
					assert.Greater(t, e.Seq, prev)
					prev = e.Seq
				}
				select {
				case <-ch:
				default:
				}
				_ = h.Status()
				cancel()
			}
		}()
	}
	wg.Wait()

	st := h.Status()
	assert.Equal(t, 100, st.Retained)
	assert.Equal(t, uint64(writers*perWriter-100), st.Dropped)
	assert.Len(t, st.Sources, writers)
}
