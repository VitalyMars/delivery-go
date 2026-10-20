# Delivery Go

Микросервисный backend сервиса доставки еды на Go.

Проект включает регистрацию пользователей, каталог ресторанов и блюд, оформление заказов, назначение курьеров и email-уведомления.

## Стек

Go, Chi, PostgreSQL, pgx, Kafka, gRPC, Protocol Buffers, Docker Compose, GitHub Actions.

## Сервисы

| Сервис | Назначение |
|---|---|
| User | Регистрация, вход и получение профиля |
| Catalog | Рестораны, меню, цены и доступность блюд |
| Order | Создание заказов, проверка позиций и управление статусами |
| Delivery | Очередь доставок и назначение курьеров |
| Notification | Отправка email-уведомлений и повторные попытки при ошибках |

Синхронное взаимодействие между сервисами организовано через gRPC, асинхронное — через Kafka. Каждый сервис использует отдельную базу PostgreSQL.

## Основные возможности

- Авторизация пользователей с помощью подписанных токенов.
- Проверка цен и доступности блюд через Catalog перед созданием заказа.
- Сохранение снимка позиций заказа.
- Идемпотентное создание заказов.
- Публикация событий через transactional outbox.
- Обработка Kafka-событий с ручным подтверждением, повторными попытками и DLQ.
- Назначение курьеров с блокировками PostgreSQL.
- Фоновая отправка email-уведомлений.
- SQL-миграции, unit-тесты и интеграционные тесты репозиториев PostgreSQL.

## Запуск

Для запуска необходимы Docker Engine и Docker Compose. Команды выполняются из корня проекта.

Создание `.env` со случайными секретами:

```powershell
.\scripts\setup-env.ps1
```

Скрипт выполняется один раз и не перезаписывает существующий `.env`.

Сборка и запуск:

```powershell
docker compose -f compose.yaml -f compose.local.yaml up --build -d
```

Проверка состояния:

```powershell
docker compose -f compose.yaml -f compose.local.yaml ps -a
```

Просмотр логов:

```powershell
docker compose -f compose.yaml -f compose.local.yaml logs -f order delivery notification
```

`compose.yaml` содержит PostgreSQL, Kafka и Mailpit. Файл `compose.local.yaml` добавляет приложения, применение миграций и создание Kafka topics.

Контейнеры `migrate` и `kafka-topics` завершаются с кодом `0` после выполнения своих задач.

## Адреса

| Компонент | Адрес |
|---|---|
| Catalog HTTP | http://localhost:8080 |
| Order HTTP | http://localhost:8081 |
| Delivery HTTP | http://localhost:8082 |
| User HTTP | http://localhost:8083 |
| Notification health | http://localhost:8084/health |
| Mailpit | http://localhost:8025 |
| PostgreSQL | localhost:5433 |
| Kafka | localhost:9092 |

Локальные параметры PostgreSQL: пользователь `app`, пароль `catalog`.

Внутри Docker-сети используются адреса `kafka:29092`, `catalog:9090` и `user:9094`.

## Данные каталога

Добавление демонстрационного ресторана и блюда:

```powershell
Get-Content .\scripts\seed.sql | docker compose exec -T postgres psql -v ON_ERROR_STOP=1 -U app -d catalog_db
```

Повторное выполнение не создаёт дубликаты. Начальная миграция Delivery добавляет трёх курьеров.

## API

### Пользователи

| Метод | Маршрут | Назначение |
|---|---|---|
| POST | `/users/register` | Регистрация |
| POST | `/users/login` | Вход |
| GET | `/users/{userID}` | Получение собственного профиля |

После входа возвращается `access_token` со сроком действия один час. Он передаётся в заголовке:

```http
Authorization: Bearer <access_token>
```

### Каталог

| Метод | Маршрут | Назначение |
|---|---|---|
| GET | `/restaurants` | Список ресторанов |
| GET | `/restaurants/{restaurantId}` | Информация о ресторане |
| GET | `/restaurants/{restaurantId}/menu` | Меню ресторана |
| GET | `/dishes/{dishId}` | Информация о блюде |

### Заказы

| Метод | Маршрут | Назначение |
|---|---|---|
| POST | `/orders` | Создание заказа |
| GET | `/orders/{orderId}` | Получение заказа |
| POST | `/orders/{orderId}/pay` | Подтверждение демонстрационной оплаты |

Пример тела запроса создания заказа:

```json
{
  "customer_name": "Test User",
  "customer_phone": "+79990000000",
  "delivery_address": "Тестовая улица, дом 10",
  "comment": "",
  "idempotency_key": "33333333-3333-4333-8333-333333333333",
  "restaurant_id": "11111111-1111-4111-8111-111111111111",
  "items": [
    {
      "dish_id": "22222222-2222-4222-8222-222222222222",
      "quantity": 1
    }
  ]
}
```

Для зарегистрированного пользователя передаётся `Authorization: Bearer <access_token>`. Идентификатор пользователя определяется из сессии.

Гостевой заказ создаётся без авторизации, со случайным UUID в заголовке `X-Checkout-Token`. При повторе запроса сохраняются тот же токен, тело и `idempotency_key`.

Для каждого нового заказа используется новый UUID в `idempotency_key`. Повтор исходного запроса возвращает ранее созданный заказ. Изменение тела с прежним ключом возвращает `409 Conflict`.

Ответ содержит `order_id` и `order_token`. Получение и оплата заказа требуют заголовок:

```http
X-Order-Token: <order_token>
```

Токен заказа передаётся именно в Headers, а не в query-параметрах URL.

### Доставка

```http
POST /deliveries/{id}/complete
X-Internal-API-Key: <INTERNAL_API_KEY>
```

Параметр `id` — UUID доставки из `delivery_db.deliveries`, а не UUID заказа. Успешное завершение возвращает `204 No Content`.

## Обработка заказа

1. Order получает позиции и проверяет их через Catalog.
2. Заказ сохраняется вместе со снимком названий и цен.
3. После подтверждения демонстрационной оплаты событие записывается в outbox.
4. Publisher отправляет `order.ready_for_delivery` в Kafka.
5. Delivery сохраняет доставку в очередь, Notification создаёт задание на письмо.
6. При наличии свободного курьера Delivery назначает его и публикует `delivery.courier_assigned`.
7. После завершения доставки событие `delivery.completed` обновляет статус заказа на `delivered`.

Письма доступны в Mailpit: http://localhost:8025.

## Миграции

Миграции выполняются отдельным контейнером перед запуском приложений. История версий и контрольные суммы сохраняются в `app_schema_migrations`.

Поддерживается перенос чистой истории golang-migrate из таблицы `schema_migrations`. Состояние `dirty=true` блокирует обновление до исправления незавершённой миграции.

Для существующих таблиц без истории предусмотрен параметр `-baseline`. Он указывает последнюю фактически применённую миграцию.

Например, если в Order уже применены версии 1–8:

```powershell
docker compose -f compose.yaml -f compose.local.yaml run --rm migrate -service order -baseline 8
```

Команда фиксирует версии 1–8 и применяет последующие. Параметр не проверяет соответствие существующей схемы SQL-файлам, поэтому версия определяется по состоянию БД. Для новой пустой БД baseline не требуется.

После переноса истории дальнейшие обновления выполняются через migration runner проекта.

## Тестирование

Запуск автоматических тестов:

```powershell
go test ./... ./services/catalog/... ./services/order/... ./services/delivery/... ./services/notification/... ./services/user/... ./gen/...
```

Статическая проверка:

```powershell
go vet ./... ./services/catalog/... ./services/order/... ./services/delivery/... ./services/notification/... ./services/user/... ./gen/...
```

Интеграционные тесты PostgreSQL используют отдельную тестовую БД:

```powershell
$env:TEST_DATABASE_URL='postgres://app:catalog@localhost:5433/test_db?sslmode=disable'

go test -count=1 ./services/order/internal/repository ./services/delivery/internal/repository ./services/notification/internal/repository ./services/user/internal/repository
```

База `test_db` создаётся заранее. Тесты работают во временных схемах; пользователь БД должен иметь права их создания и удаления.

Без `TEST_DATABASE_URL` интеграционные тесты PostgreSQL пропускаются. GitHub Actions запускает PostgreSQL, тесты и race detector.

## Ограничения

- Оплата демонстрационная: endpoint `/pay` доступен при `DEMO_PAYMENTS=true`, платёжный провайдер не подключён.
- Готовность заказа к доставке наступает после демонстрационной оплаты; этап приготовления отдельно не моделируется.
- SMTP настроен для локального Mailpit.
- Kafka и внутренний gRPC в локальной конфигурации работают без TLS.
- Серверная корзина отсутствует: клиент передаёт выбранные позиции напрямую при создании заказа.
