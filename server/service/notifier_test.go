package service_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/alwayson/server/service"
	"github.com/stretchr/testify/assert"
)

func TestSlackNotifier_Send(t *testing.T) {
	received := ""
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf := make([]byte, 1024)
		n, _ := r.Body.Read(buf)
		received = string(buf[:n])
		w.WriteHeader(200)
	}))
	defer ts.Close()

	n := service.NewSlackNotifier(ts.URL)
	err := n.Send("테스트 알림 메시지")
	assert.NoError(t, err)
	assert.Contains(t, received, "테스트 알림 메시지")
}

func TestSlackNotifier_EmptyURL_PrintsToConsole(t *testing.T) {
	n := service.NewSlackNotifier("")
	err := n.Send("콘솔 출력 테스트")
	assert.NoError(t, err)
}
