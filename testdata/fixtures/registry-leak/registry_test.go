package registryleak

import "testing"

func TestCatalog(t *testing.T) {
	if model := clients["websocket"]; model != "" {
		t.Fatalf("unexpected model in catalog: %s", model)
	}
}

func TestWebsocketClient(t *testing.T) { register("websocket", "model-with-extra-capabilities") }
