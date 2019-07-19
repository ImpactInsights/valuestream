package jenkins

import (
	"encoding/json"
	log "github.com/sirupsen/logrus"
	"net/http"
)

type tracer interface {
	handleBuild(*BuildEvent) error
}

type Webhook struct {
	et tracer
}

func (hook *Webhook) HTTPBuildHandler(w http.ResponseWriter, r *http.Request) {
	decoder := json.NewDecoder(r.Body)
	var build BuildEvent
	err := decoder.Decode(&build)
	if err != nil {
		log.Infof("Error reading body: %v", err)
		http.Error(w, "can't read body", http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if err := hook.et.handleBuild(&build); err != nil {
		log.Warnf("Error handling build event: %v", err)
		http.Error(w, "error", http.StatusBadRequest)
		return
	}

	s, _ := build.String()
	log.Infof("Body: %q", s)
	w.Write([]byte("success"))
}

func NewWebhook(et tracer) *Webhook {
	return &Webhook{
		et: et,
	}
}
