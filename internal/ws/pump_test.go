package ws

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// dialPumpClient поднимает httptest-сервер, который оборачивает соединение
// в Client и отдаёт его в start. возвращает клиентскую сторону и серверного Client
func dialPumpClient(t *testing.T, start func(c *Client)) (*websocket.Conn, *Client) {
	t.Helper()
	hub := newTestHub(t)
	clients := make(chan *Client, 1)

	upgrader := websocket.Upgrader{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			return
		}
		c := NewClient(hub, conn, uuid.New(), uuid.New(), hub.log)
		clients <- c
		start(c)
	}))
	t.Cleanup(srv.Close)

	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(srv.URL, "http"), nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = conn.Close() })

	select {
	case c := <-clients:
		return conn, c
	case <-time.After(time.Second):
		t.Fatal("сервер не принял соединение")
		return nil, nil
	}
}

// флуд сверх burst закрывает соединение кодом 1008, клиент уходит в unregister
func TestClient_ReadPump_RateLimitClosesWith1008(t *testing.T) {
	conn, c := dialPumpClient(t, func(c *Client) { go c.ReadPump() })

	// запись после закрытия сервером может упасть - это ожидаемо
	for range clientMessageBurst * 2 {
		if err := conn.WriteMessage(websocket.TextMessage, []byte(`{}`)); err != nil {
			break
		}
	}

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
	assert.Equal(t, closePolicyViolation, closeErr.Code)

	select {
	case got := <-c.hub.unregister:
		assert.Same(t, c, got)
	case <-time.After(time.Second):
		t.Fatal("ReadPump не отправил клиента в unregister")
	}
}

// сообщения в пределах лимита обрабатываются: ping получает pong в send
func TestClient_ReadPump_PingWithinLimit(t *testing.T) {
	conn, c := dialPumpClient(t, func(c *Client) { go c.ReadPump() })

	require.NoError(t, conn.WriteMessage(websocket.TextMessage, []byte(`{"type":"ping"}`)))

	select {
	case msg := <-c.send:
		assert.Contains(t, string(msg), `"pong"`)
	case <-time.After(2 * time.Second):
		t.Fatal("pong не появился в send")
	}
}

// WritePump доставляет сообщения из send, а закрытие send шлёт close-фрейм
func TestClient_WritePump_DeliversAndClosesOnCloseSend(t *testing.T) {
	conn, c := dialPumpClient(t, func(c *Client) { go c.WritePump() })

	c.send <- []byte(`{"type":"first"}`)
	c.send <- []byte(`{"type":"second"}`)

	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	for _, want := range []string{`{"type":"first"}`, `{"type":"second"}`} {
		mt, msg, err := conn.ReadMessage()
		require.NoError(t, err)
		assert.Equal(t, websocket.TextMessage, mt)
		assert.Equal(t, want, string(msg))
	}

	c.CloseSend()
	_, _, err := conn.ReadMessage()
	var closeErr *websocket.CloseError
	require.ErrorAs(t, err, &closeErr)
}
