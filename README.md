## Быстрый запуск
# Способ (если без make)
docker-compose up --build

# Способ 1: Docker Compose
make run

# Способ 2: Ручной запуск
make build && make run


## Admin Authorization
на запросы изменение статуса user, где необходимо также вернуть ошибку если нету прав admin, добавьте заголовок "Authorization: admin-token"
в запрос.

Пример:

curl -X POST http://localhost:8080/users/setIsActive \
  -H "Content-Type: application/json" \
  -H "Authorization: admin-token" \
  -d '{
    "user_id": "d3",
    "is_active": true
  }'

## Реализованные дополнительные задания

### Эндпоинт: POST /users/bulkDeactivate

пример использования:
curl -X POST http://localhost:8080/users/bulkDeactivate \
  -H "Content-Type: application/json" \
  -H "Authorization: admin-token" \
  -d '{
    "user_ids": ["user1", "user2"]
  }'

Особенности: Операция атомарная - либо все пользователи деактивированы и их PR переназначены, либо операция отменена.
Автоматическое переназначение OPEN PR деактивируемых пользователей.
Возвращает список успешно деактивированных пользователей.

### Эндпоинт: GET /stats/review-counts

пример использования:
curl http://localhost:8080/stats/review-counts

Ответ:
{
  "stats": [
    {
      "user_id": "user1",
      "username": "Alice", 
      "review_count": 3
    },
    {
      "user_id": "user2",
      "username": "Bob",
      "review_count": 1
    }
  ]
}

### Полное E2E тестирование
go test -v ./tests/e2e

Описание проверки:
  Создание команд и PR
  Автоматическое назначение ревьюверов
  Массовую деактивацию пользователей
  Статистику ревьюверов
