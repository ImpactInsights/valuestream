package traces

import (
	"context"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestBufferedSpans_Set_OverBuffer_Bounded(t *testing.T) {
	ctx := context.Background()
	spans, err := NewBufferedSpanStore(1)
	assert.NoError(t, err)

	tracer := mocktracer.New()
	entry1 := StoreEntry{
		Span: tracer.StartSpan("span1"),
	}

	entry2 := StoreEntry{
		Span: tracer.StartSpan("span2"),
	}

	err = spans.Set(ctx, "span1", entry1)
	assert.NoError(t, err)

	err = spans.Set(ctx, "span2", entry2)
	assert.EqualError(t, err, "maxAllowedSpans: 1 reached")

	c, _ := spans.Count()
	assert.Equal(t, 1, c)

	// check that the span is the first span
	e1, err := spans.Get(ctx, tracer, "span1")
	assert.NoError(t, err)
	assert.Equal(t, entry1, *e1)
}

func TestBufferedSpans_Delete(t *testing.T) {
	ctx := context.Background()
	spans, err := NewBufferedSpanStore(1)
	assert.NoError(t, err)

	tracer := mocktracer.New()
	entry1 := StoreEntry{
		Span: tracer.StartSpan("span1"),
	}
	err = spans.Set(ctx, "span1", entry1)
	assert.NoError(t, err)
	err = spans.Delete(ctx, "span1")
	assert.NoError(t, err)

	c, _ := spans.Count()

	assert.Equal(t, 0, c)
}
