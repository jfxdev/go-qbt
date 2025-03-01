package request

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Testa a opção WithTimeout
func TestWithTimeout(t *testing.T) {
	timeout := 2 // segundos

	// Cria um servidor de teste que simula um delay na resposta
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(1 * time.Second) // Simula um pequeno delay (menor que o timeout)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Faz a requisição com timeout
	resp, err := Do(http.MethodPost, server.URL, WithTimeout(timeout))
	if err != nil {
		t.Fatalf("Erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	// Verifica se a resposta foi 200 OK
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Esperado status 200, mas recebeu %d", resp.StatusCode)
	}
}

// Testa a opção WithBody
func TestWithBody(t *testing.T) {
	expectedBody := `{"message": "hello"}`

	// Servidor de teste para capturar o corpo da requisição
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if string(body) != expectedBody {
			t.Errorf("Esperado body '%s', mas recebeu '%s'", expectedBody, string(body))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Envia a requisição com corpo
	_, err := Do(http.MethodPost, server.URL, WithBody(strings.NewReader(expectedBody)))
	if err != nil {
		t.Fatalf("Erro na requisição: %v", err)
	}
}

// Testa a opção WithHeader
func TestWithHeader(t *testing.T) {
	expectedKey := "X-Custom-Header"
	expectedValue := "test-value"

	// Servidor de teste para capturar cabeçalhos
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(expectedKey) != expectedValue {
			t.Errorf("Esperado header '%s' com valor '%s', mas recebeu '%s'", expectedKey, expectedValue, r.Header.Get(expectedKey))
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Envia a requisição com um cabeçalho customizado
	_, err := Do(http.MethodPost, server.URL, WithHeader(expectedKey, expectedValue))
	if err != nil {
		t.Fatalf("Erro na requisição: %v", err)
	}
}

// Testa a opção WithHeaders (múltiplos cabeçalhos)
func TestWithHeaders(t *testing.T) {
	expectedHeaders := map[string]string{
		"X-Header-One": "value1",
		"X-Header-Two": "value2",
	}

	// Servidor de teste para capturar múltiplos cabeçalhos
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		for k, v := range expectedHeaders {
			if r.Header.Get(k) != v {
				t.Errorf("Esperado header '%s' com valor '%s', mas recebeu '%s'", k, v, r.Header.Get(k))
			}
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Envia a requisição com múltiplos cabeçalhos
	_, err := Do(http.MethodPost, server.URL, WithHeaders(expectedHeaders))
	if err != nil {
		t.Fatalf("Erro na requisição: %v", err)
	}
}
