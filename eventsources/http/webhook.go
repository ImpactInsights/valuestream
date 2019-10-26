package http

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"fmt"
	"github.com/ImpactInsights/valuestream/eventsources/webhooks"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

func ValidatePayload(r *http.Request, secretKey []byte) ([]byte, error) {
	var body []byte
	var err error

	if body, err = ioutil.ReadAll(r.Body); err != nil {
		return nil, fmt.Errorf("unable to read body")
	}

	sig := r.Header.Get(webhooks.SignatureHeader)
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write(body)
	expectedMAC := mac.Sum(nil)
	if !hmac.Equal([]byte(sig), expectedMAC) {
		return nil, fmt.Errorf("invalid event signature")
	}

	return body, nil
}

type Webhook struct {
	HandleEvent func(ctx context.Context, payload []byte) error
}

func (wh *Webhook) Handler(w http.ResponseWriter, r *http.Request) {
	var payload []byte
	var err error
	var secretKey []byte

	k := r.Context().Value(webhooks.CtxSecretTokenKey)
	v, ok := k.([]byte)
	if ok && v != nil {
		secretKey = v
	}

	if payload, err = ValidatePayload(r, secretKey); err != nil {
		log.WithFields(log.Fields{
			"error":   err.Error(),
			"payload": payload,
		}).Errorf("unable to parse webhook event")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := wh.HandleEvent(r.Context(), payload); err != nil {
		log.Errorf("Error handling event: %v", err)
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	w.Write([]byte("success"))
}
