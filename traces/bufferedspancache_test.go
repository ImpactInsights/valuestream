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
	span1 := tracer.StartSpan("span1")
	span2 := tracer.StartSpan("span2")

	spans.Set(ctx, "span1", span1)
	spans.Set(ctx, "span2", span2)
	c, _ := spans.Count()
	assert.Equal(t, 1, c)

	// check that the span is the second span
	s2, err := spans.Get(ctx, "span2")
	assert.NoError(t, err)
	assert.Equal(t, span2, s2)
}

func TestBufferedSpans_Delete(t *testing.T) {
	ctx := context.Background()
	spans, err := NewBufferedSpanStore(1)
	assert.NoError(t, err)

	tracer := mocktracer.New()
	span1 := tracer.StartSpan("span1")
	spans.Set(ctx, "span1", span1)
	spans.Delete(ctx, "span1")

	c, _ := spans.Count()

	assert.Equal(t, 0, c)
	assert.Nil(t, spans.buf[0])
}
