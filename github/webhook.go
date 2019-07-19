package github

import (
	"github.com/ImpactInsights/valuestream/traces"
	"github.com/google/go-github/github"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

type tracer interface {
	handleEvent(e interface{}) error
}

type Webhook struct {
	et          tracer
	secretToken []byte
}

func NewWebhook(et tracer, secretToken []byte) *Webhook {
	return &Webhook{
		et:          et,
		secretToken: secretToken,
	}
}

func (webhook *Webhook) Handler(w http.ResponseWriter, r *http.Request) {
	var err error
	var payload []byte

	if webhook.secretToken != nil {
		payload, err = github.ValidatePayload(r, webhook.secretToken)
	} else {
		payload, err = ioutil.ReadAll(r.Body)
	}

	log.WithFields(log.Fields{
		"event": string(payload),
	}).Debug("received_event")

	if err != nil {
		log.WithFields(log.Fields{
			"error": err.Error(),
		}).Errorf("unable to parse webhook event")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	defer r.Body.Close()

	event, err := github.ParseWebHook(github.WebHookType(r), payload)
	if err != nil {
		log.WithFields(log.Fields{
			"raw":   string(payload),
			"error": err.Error(),
		}).Errorf("unable to parse webhook event")
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	if err = webhook.et.handleEvent(event); err != nil {
		switch err.(type) {
		case traces.SpanMissingError:
			log.WithFields(log.Fields{
				"error": err,
			}).Warnf("event not handled, %s", err)
		case traces.SpanMissingIDError:
			log.WithFields(log.Fields{
				"error": err,
			}).Warnf("event not handled, %s", err)
		default:
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("event not handled, %s", err)
		}
	}

	w.Write([]byte("success"))
}
