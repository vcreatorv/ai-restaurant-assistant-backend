# Сертификаты

## russian_trusted_bundle.pem

Объединённый PEM-bundle корневого и выпускающего сертификатов Минцифры:

```
russian_trusted_sub_ca.cer  +  russian_trusted_root_ca.cer  →  russian_trusted_bundle.pem
```

Нужен для TLS-соединения с GigaChat (`gigachat.devices.sberbank.ru`,
`ngw.devices.sberbank.ru`) — эти домены подписаны Минцифры, которой
нет в дефолтном trust store Linux/Alpine.

Bundle подгружается **только** в `internal/pkg/gigachat`-клиенте через
`tls.Config.RootCAs`. Остальные HTTPS-вызовы (NVIDIA, OpenRouter,
Cohere, MinIO, Qdrant) используют системный trust store без изменений.

## Обновление

Сертификаты раздаёт Минцифры публично, версии обновляются раз в год.
Источник: <https://www.gosuslugi.ru/crt> или
<https://developers.sber.ru/docs/ru/gigachat/certificates>.

Чтобы обновить:

```sh
curl -sSL -o certs/russian_trusted_root_ca.cer \
  https://gu-st.ru/content/Other/doc/russian_trusted_root_ca.cer
curl -sSL -o certs/russian_trusted_sub_ca.cer \
  https://gu-st.ru/content/Other/doc/russian_trusted_sub_ca.cer

# Важно: между файлами обязательно ставить перевод строки, иначе Go pem.Decode
# увидит "-----END CERTIFICATE----------BEGIN CERTIFICATE-----" слипшимся и не
# распознает блок. Также убираем \r — на Windows файлы могут быть с CRLF.
(cat certs/russian_trusted_sub_ca.cer | tr -d '\r'; echo; \
 cat certs/russian_trusted_root_ca.cer | tr -d '\r') \
  > certs/russian_trusted_bundle.pem
```
