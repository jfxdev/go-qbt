package request

import (
	"context"
	"io"
	"net/http"
	"time"
)

// Estrutura que contém as configurações da requisição
type RequestOptions struct {
	Timeout        time.Duration
	Body           io.Reader
	Headers        map[string]string
	Ctx            context.Context
	CookieJar      http.CookieJar
	UpdateCookies  bool
	PreRequestHook func() error
}

// Tipo de função para aplicar opções à RequestOptions
type RequestOption func(*RequestOptions)

// WithTimeout define um tempo limite para a requisição
func WithTimeout(seconds int) RequestOption {
	return func(o *RequestOptions) {
		o.Timeout = time.Duration(seconds) * time.Second
	}
}

// WithBody define um corpo para a requisição
func WithBody(body io.Reader) RequestOption {
	return func(o *RequestOptions) {
		o.Body = body
	}
}

// WithHeader adiciona um cabeçalho à requisição
func WithHeader(key, value string) RequestOption {
	return func(o *RequestOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		o.Headers[key] = value
	}
}

// Adiciona múltiplos cabeçalhos de uma vez
func WithHeaders(headers map[string]string) RequestOption {
	return func(o *RequestOptions) {
		if o.Headers == nil {
			o.Headers = make(map[string]string)
		}
		for k, v := range headers {
			o.Headers[k] = v
		}
	}
}

// WithContext permite definir um contexto para a requisição
func WithContext(ctx context.Context) RequestOption {
	return func(o *RequestOptions) {
		o.Ctx = ctx
	}
}

// Usa um CookieJar para armazenar cookies entre requisições
func WithCookieJar(jar http.CookieJar) RequestOption {
	return func(o *RequestOptions) {
		o.CookieJar = jar
	}
}

// Define um hook que será executado antes da requisição
func WithUpdateCookies() RequestOption {
	return func(o *RequestOptions) {
		o.UpdateCookies = true
	}
}

// Define um hook que será executado antes da requisição
func WithPreRequestHook(hook func() error) RequestOption {
	return func(o *RequestOptions) {
		o.PreRequestHook = hook
	}
}

// Execute a HTTP request com opções personalizadas
func Do(method, url string, opts ...RequestOption) (*http.Response, error) {
	// Configuração padrão
	options := &RequestOptions{
		Timeout: 10 * time.Second, // Default de 10s
		Ctx:     context.Background(),
		Body:    nil,
	}

	// Aplicar todas as opções passadas
	for _, opt := range opts {
		opt(options)
	}

	// Criar um cliente HTTP com timeout configurado
	client := &http.Client{Timeout: options.Timeout}

	// Se houver um CookieJar, usa ele no client
	if options.CookieJar != nil {
		client.Jar = options.CookieJar
	}

	// Criar a requisição
	req, err := http.NewRequestWithContext(options.Ctx, http.MethodPost, url, options.Body)
	if err != nil {
		return nil, err
	}

	// Adicionar cabeçalhos
	for k, v := range options.Headers {
		req.Header.Set(k, v)
	}

	// Executar a requisição
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Salva os cookies da resposta no CookieJar
	if options.UpdateCookies {
		options.CookieJar.SetCookies(req.URL, resp.Cookies())
	}

	// Executar a requisição
	return resp, nil
}
