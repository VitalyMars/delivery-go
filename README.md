# Delivery Go

Сервис доставки еды: Go, PostgreSQL, gRPC, Kafka, transactional outbox и отправка уведомлений через Mailpit.

## Быстрый запуск

Нужен запущенный Docker Engine с Docker Compose. Команды выполняются из корня репозитория.

```powershell
# Один раз: создаёт .env со случайными секретами, существующий файл не перезаписывает.
.\scripts\setup-env.ps1

docker compose -f compose.yaml -f compose.local.yaml up --build -d
docker compose -f compose.yaml -f compose.local.yaml ps -a
docker compose -f compose.yaml -f compose.local.yaml logs -f order delivery notification
```

`compose.yaml` оставлен для запуска одной инфраструктуры; `compose.local.yaml` добавляет приложения, создание БД, миграции и Kafka topics. При объединённом запуске внешние HTTP-порты приложений привязаны к localhost. Это локальная среда, Kafka и SMTP работают без production TLS/authentication.

| Компонент | Адрес на хосте |
|---|---|
| Catalog HTTP | http://localhost:8080 |
| Order HTTP | http://localhost:8081 |
| Delivery HTTP | http://localhost:8082 |
| User HTTP | http://localhost:8083 |
| Notification health | http://localhost:8084/health |
| Mailpit | http://localhost:8025 |
| PostgreSQL | localhost:5433, `app` / `catalog` |
| Kafka для программ на хосте | localhost:9092 |

Внутри Compose приложения используют `kafka:29092`, gRPC-адреса `catalog:9090` и `user:9094`.

### Если БД уже существовала

Не удаляй volume ради обновления. Новый migration runner ведёт историю в `app_schema_migrations`, сохраняя прежнюю `schema_migrations` без изменений. Чистая история golang-migrate (`version`, `dirty=false`) переносится автоматически. При `dirty=true` запуск останавливается до исправления незавершённой миграции. После перехода используй новый runner: старый инструмент больше не отражает новые версии.

Если таблицы созданы вручную и истории нет, runner откажется повторно применять начальные миграции.

Проверь, какие SQL-миграции реально применены. Для каждого существующего сервиса зафиксируй **последнюю применённую** версию:

```powershell
# Пример: только если у Order уже применены миграции 1..8.
$env:DATABASE_URL='postgres://app:catalog@localhost:5433/postgres?sslmode=disable'
go run ./cmd/migrate -service order -baseline 8
```

Эта команда запишет историю 1..8 и применит последующие миграции. `-baseline` не проверяет соответствие существующих таблиц SQL-файлам: версию необходимо выбрать по фактической схеме. Аналогично обработай остальные существующие БД. После этого обычный запуск сможет продолжить работу. Для пустой новой БД baseline не нужен.

При запуске мигратора внутри Compose используй `docker compose -f compose.yaml -f compose.local.yaml run --rm migrate -service order -baseline 8` с проверенными именем сервиса и версией.

### Тестовые данные каталога

```powershell
Get-Content .\scripts\seed.sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d catalog_db
```

Миграции Delivery сохраняют прежнее поведение: создают трёх демонстрационных курьеров.

## Пример заказа

```powershell
.\scripts\load-env.ps1
$email = "student-$([guid]::NewGuid().ToString('N'))@example.com"
$credentials = @{name='Student';email=$email;password='learning-password-123'} | ConvertTo-Json
Invoke-RestMethod http://localhost:8083/users/register -Method Post -ContentType application/json -Body $credentials
$session = Invoke-RestMethod http://localhost:8083/users/login -Method Post -ContentType application/json -Body $credentials
$headers = @{Authorization="Bearer $($session.access_token)"}
$body = @{
    customer_name='Student'
    customer_phone='+79990000000'
    delivery_address='Demo street, 2'
    idempotency_key=[guid]::NewGuid().ToString()
    restaurant_id='11111111-1111-4111-8111-111111111111'
    items=@(@{dish_id='22222222-2222-4222-8222-222222222222';quantity=1})
} | ConvertTo-Json -Depth 4
$order = Invoke-RestMethod http://localhost:8081/orders -Method Post -Headers $headers -ContentType application/json -Body $body
$orderHeaders = @{'X-Order-Token'=$order.order_token}
Invoke-RestMethod "http://localhost:8081/orders/$($order.order_id)/pay" -Method Post -Headers $orderHeaders
Invoke-RestMethod "http://localhost:8081/orders/$($order.order_id)" -Headers $orderHeaders
```

В `.env` учебного запуска включён `DEMO_PAYMENTS=true`. Endpoint оплаты имитирует подтверждение и не обращается к платёжной системе. При отсутствии этого флага он выключен.

После оплаты worker назначит курьера примерно в течение 5 секунд. Письмо появится в Mailpit. Для завершения доставки возьми её `id` из `delivery_db.deliveries` и отправь:

```powershell
# Подставь реальный UUID доставки.
$deliveryId='REPLACE_WITH_DELIVERY_UUID'
Invoke-RestMethod "http://localhost:8082/deliveries/$deliveryId/complete" -Method Post -Headers @{'X-Internal-API-Key'=$env:INTERNAL_API_KEY}
```

Order получит `delivery.completed` асинхронно и обновит статус на `delivered`.

## Изменения API

- Login выдаёт `access_token` на один час. Профиль доступен только владельцу с `Authorization: Bearer ...`.
- `user_id` заказа выводится из сессии; подмена значения в JSON запрещена.
- Гостевой заказ требует случайный UUID в `X-Checkout-Token`. Сохраняй его для повторов своего checkout.
- `idempotency_key` — обязательный непустой UUID. Для повтора сохраняй тело и идентификатор клиента; другое тело с тем же ключом вернёт 409.
- Токен заказа передаётся в `X-Order-Token`. Query-параметры с токенами больше не используются.
- Серверная корзина удалена: клиент хранит выбранные блюда и передаёт их в `items` запроса `POST /orders`. Order проверяет доступность и получает актуальные цены из Catalog.
- gRPC и завершение доставки требуют `INTERNAL_API_KEY`. При работе через Go-клиенты ключ передаётся interceptor автоматически.
- Неизвестные JSON-поля, несколько JSON-документов и тело свыше 1 MiB отклоняются.

Маршруты `/cart` и `/cart/items` больше не доступны. Исторические миграции и таблицы `carts`/`cart_items` сохранены для совместимости с существующей БД; приложение их больше не использует. Удаление этих данных при необходимости оформляется отдельной миграцией.

## Проверки

```powershell
go test ./... ./services/catalog/... ./services/order/... ./services/delivery/... ./services/notification/... ./services/user/... ./gen/...
go vet ./... ./services/catalog/... ./services/order/... ./services/delivery/... ./services/notification/... ./services/user/... ./gen/...
```

Для интеграционных тестов используй отдельную тестовую PostgreSQL БД. Тестовый пользователь должен иметь права создания и удаления схем:

```powershell
$env:TEST_DATABASE_URL='postgres://app:catalog@localhost:5433/test_db?sslmode=disable'
go test -count=1 ./services/order/internal/repository ./services/delivery/internal/repository ./services/notification/internal/repository ./services/user/internal/repository
```

Без `TEST_DATABASE_URL` эти тесты явно пропускаются. Они работают во временных случайных схемах; `test_db` нужно создать заранее. CI запускает PostgreSQL и выполняет интеграционные тесты и `go test -race`.

## Локальная разработка без контейнеров приложений

```powershell
docker compose up -d
.\scripts\load-env.ps1
go run ./cmd/migrate
# После создания Kafka topics запусти каждый сервис в отдельном терминале:
go run ./services/catalog/cmd/catalog
go run ./services/user/cmd/user
go run ./services/order/cmd/order
go run ./services/delivery/cmd/delivery
go run ./services/notification/cmd/notification
```

Каждый терминал должен загрузить переменные из `.env`. Нужные топики: `order.ready_for_delivery`, `delivery.courier_assigned`, `delivery.completed` и отдельный `<topic>.dlq` для каждого из них. Если до этого применялся `compose.local.yaml`, для внутреннего listener используй `kafka:29092`, для программ на хосте — `localhost:9092`.
