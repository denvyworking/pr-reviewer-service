set -e

echo "Waiting for PostgreSQL to start..."
until pg_isready -h postgres -p 5432 -U user; do
  sleep 1
done

echo "Running database migrations..."
psql -h postgres -p 5432 -U user -d pr_reviewer -f /migrations/001_init.sql

echo "Database initialized successfully!"