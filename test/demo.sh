#!/bin/sh
set -eu

if [ -z "${QAHYA_JOB_API:-}" ]; then
  echo "demo.sh: run me through qahyacli:  ./bin/qahyacli run test/script.yaml" >&2
  exit 1
fi

echo "== 1. export the whole staging profile into this shell"
eval "$(qahyacli env load example.config --profile staging --export)"
echo "APP_ENV   = $APP_ENV"
echo "DB_HOST   = $DB_HOST"
echo "VERSION   = $VERSION"
echo "DB_PASS   = $DB_PASS        <- secret, read from secrets.yaml"
echo "API_TOKEN = $API_TOKEN"
echo "TRICKY    = $TRICKY"

echo
echo "== 2. use them"
DATABASE_URL="postgres://app:$DB_PASS@$DB_HOST:5432/app"
echo "connecting to $DATABASE_URL"
echo "deploying version $VERSION to $APP_ENV"

echo
echo "== 3. load prod into the store, but export only DB_PASS into this shell"
qahyacli env load example.config --profile prod
DB_PASS="$(qahyacli env get DB_PASS)" && export DB_PASS
echo "DB_PASS   = $DB_PASS        <- now the prod password"
echo "APP_ENV   = $APP_ENV        <- still staging in this shell"
echo "store has APP_ENV = $(qahyacli env get APP_ENV)"

echo
echo "== 4. hand values to later steps"
qahyacli env set DEPLOYED_VERSION="$VERSION"
export LOCAL_ONLY=yes
echo "set DEPLOYED_VERSION in the store; exported LOCAL_ONLY in this shell only"
env
