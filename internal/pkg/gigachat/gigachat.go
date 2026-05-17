// Package gigachat — фабрика llm-клиента для GigaChat (Сбер).
//
// Отличия от NVIDIA/OpenRouter:
//
//   - OAuth2 client_credentials с короткоживущим access_token (~30 минут);
//     токен обновляется лениво через TokenProvider при первом запросе после
//     истечения. Refresh выполняется за 60 секунд до фактического expiry,
//     чтобы не упереться в граничные случаи.
//   - TLS с корневыми сертификатами Минцифры. Стандартный trust store их не
//     содержит, поэтому загружаем PEM-bundle (см. certs/russian_trusted_bundle.pem)
//     и подмешиваем в tls.Config.RootCAs только для GigaChat-соединений.
package gigachat

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/llm"
	"github.com/example/ai-restaurant-assistant-backend/internal/pkg/logger"
	"github.com/example/ai-restaurant-assistant-backend/internal/rag"

	"github.com/google/uuid"
)

// ErrEmptyAuthKey AuthorizationKey не задан.
var ErrEmptyAuthKey = errors.New("gigachat: empty authorization key")

// refreshBefore насколько раньше истинного expiry мы рефрешим токен, чтобы
// избежать race'ов с граничными случаями (часовой дрейф, сетевые задержки).
const refreshBefore = 60 * time.Second

// New собирает llm.Client с OAuth-токен-менеджером и кастомным TLS-конфигом.
func New(cfg rag.GigaChatConfig, common rag.LLMCommonConfig) (*llm.Client, error) {
	if cfg.AuthorizationKey == "" {
		return nil, ErrEmptyAuthKey
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = "https://gigachat.devices.sberbank.ru/api/v1"
	}
	if cfg.AuthURL == "" {
		cfg.AuthURL = "https://ngw.devices.sberbank.ru:9443/api/v2/oauth"
	}
	if cfg.Scope == "" {
		cfg.Scope = "GIGACHAT_API_PERS"
	}

	tlsConfig, err := buildTLSConfig(cfg.CABundlePath)
	if err != nil {
		return nil, fmt.Errorf("gigachat: tls: %w", err)
	}

	transport := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSClientConfig:       tlsConfig,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient := &http.Client{Timeout: common.RequestTimeout, Transport: transport}

	tokenMgr := &tokenManager{
		authURL:          cfg.AuthURL,
		authorizationKey: cfg.AuthorizationKey,
		scope:            cfg.Scope,
		http:             httpClient,
	}

	return llm.New(llm.Config{
		Provider:          "gigachat",
		BaseURL:           cfg.BaseURL,
		TokenProvider:     tokenMgr.Token,
		HTTPClient:        httpClient,
		Model:             cfg.Model,
		Temperature:       common.Temperature,
		MaxTokens:         common.MaxTokens,
		RequestTimeout:    common.RequestTimeout,
		FirstTokenTimeout: common.FirstTokenTimeout,
	})
}

// buildTLSConfig создаёт TLS-конфиг с подмешанными CA Минцифры.
//
// Берём системный CA pool (чтобы запросы на github / прочее работали из этого же
// клиента, если такое вдруг понадобится), и добавляем туда наш bundle.
func buildTLSConfig(bundlePath string) (*tls.Config, error) {
	if bundlePath == "" {
		return nil, fmt.Errorf("ca_bundle_path is empty (укажите путь к russian_trusted_bundle.pem)")
	}
	pem, err := os.ReadFile(bundlePath)
	if err != nil {
		return nil, fmt.Errorf("read ca bundle %q: %w", bundlePath, err)
	}
	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if !pool.AppendCertsFromPEM(pem) {
		return nil, fmt.Errorf("ca bundle %q does not contain any PEM certificates", bundlePath)
	}
	return &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, nil
}

// tokenManager держит access_token в памяти и обновляет его лениво.
//
// Потокобезопасно: первый запрос после истечения захватывает мьютекс,
// остальные ждут результата. Если запрос за токеном падает — следующая
// попытка пройдёт через mutex заново (не кэшируем ошибку).
type tokenManager struct {
	authURL          string
	authorizationKey string
	scope            string
	http             *http.Client

	mu          sync.Mutex
	accessToken string
	expiresAt   time.Time
}

// Token возвращает действительный access_token, при необходимости обновляя его.
// Сигнатура совместима с llm.TokenProvider.
func (m *tokenManager) Token(ctx context.Context) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.accessToken != "" && time.Now().Before(m.expiresAt.Add(-refreshBefore)) {
		return m.accessToken, nil
	}
	if err := m.refreshLocked(ctx); err != nil {
		return "", err
	}
	return m.accessToken, nil
}

// gigaTokenResponse ответ OAuth-эндпоинта.
type gigaTokenResponse struct {
	AccessToken string `json:"access_token"`
	// ExpiresAt — UNIX-таймстемп в МИЛЛИСЕКУНДАХ (особенность Сбера). Не путать с expires_in.
	ExpiresAt int64 `json:"expires_at"`
}

// refreshLocked делает POST на authURL за новым токеном. Вызывать только под m.mu.
func (m *tokenManager) refreshLocked(ctx context.Context) error {
	log := logger.ForCtx(ctx).With("service", "gigachat", "op", "oauth")

	form := url.Values{}
	form.Set("scope", m.scope)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.authURL, strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build oauth request: %w", err)
	}
	// RqUID обязателен по спецификации Сбера; используем uuid v4.
	req.Header.Set("RqUID", uuid.NewString())
	req.Header.Set("Authorization", "Basic "+m.authorizationKey)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	t := time.Now()
	resp, err := m.http.Do(req)
	dur := time.Since(t).Milliseconds()
	if err != nil {
		log.Warn("oauth http failed", "err", err, "duration_ms", dur)
		return fmt.Errorf("oauth http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("oauth read body: %w", err)
	}
	if resp.StatusCode/100 != 2 {
		log.Warn("oauth non-2xx", "status", resp.StatusCode, "body_snippet", snippet(body))
		return fmt.Errorf("oauth status %d: %s", resp.StatusCode, snippet(body))
	}

	var parsed gigaTokenResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fmt.Errorf("oauth parse: %w", err)
	}
	if parsed.AccessToken == "" {
		return fmt.Errorf("oauth: empty access_token in response")
	}
	m.accessToken = parsed.AccessToken
	m.expiresAt = time.UnixMilli(parsed.ExpiresAt)
	log.Info("oauth ok",
		"status", resp.StatusCode,
		"duration_ms", dur,
		"expires_at", m.expiresAt.Format(time.RFC3339),
	)
	return nil
}

func snippet(b []byte) string {
	const n = 200
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
