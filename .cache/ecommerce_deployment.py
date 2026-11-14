from pathlib import Path
api=Path('F:/Afrigoals/Afrigoals_ecommerce_api');web=Path('F:/Afrigoals/Afrigoals_ecommerce')
def put(root,name,text):
 p=root/name;p.parent.mkdir(parents=True,exist_ok=True);p.write_text(text,encoding='utf-8')
p=web/'Dockerfile';s=p.read_text(encoding='utf-8').replace('COPY package.json package-lock.json ./\nRUN npm ci','COPY package.json pnpm-lock.yaml ./\nRUN corepack prepare pnpm@10.34.7 --activate && pnpm install --frozen-lockfile')
s=s.replace('ARG NEXT_PUBLIC_GOOGLE_MAP_ID','ARG NEXT_PUBLIC_GOOGLE_CLIENT_ID\nARG NEXT_PUBLIC_MOBILE_MONEY_ENABLED=true\nARG NEXT_PUBLIC_GOOGLE_MAP_ID',1).replace('ENV NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \\', 'ENV NEXT_PUBLIC_GOOGLE_CLIENT_ID=$NEXT_PUBLIC_GOOGLE_CLIENT_ID \\\n    NEXT_PUBLIC_MOBILE_MONEY_ENABLED=$NEXT_PUBLIC_MOBILE_MONEY_ENABLED \\\n    NEXT_PUBLIC_API_URL=$NEXT_PUBLIC_API_URL \\',1);put(web,'Dockerfile',s)
for root in [api,web]:
 p=root/'.dockerignore';s=p.read_text(encoding='utf-8');s+='\n.cache/\ntmp/\noutput/\n*.log\ndeploy/*.env\ndeploy/api.env\ndeploy/web.env\n';put(root,'.dockerignore',s)
p=web/'lib/api/backend.ts';s=p.read_text(encoding='utf-8').replace('env: EnvLike = process.env','env: EnvLike = { AFRIGOALS_API_URL: process.env.AFRIGOALS_API_URL, NEXT_PUBLIC_API_URL: process.env.NEXT_PUBLIC_API_URL }');put(web,'lib/api/backend.ts',s)
put(web,'app/api/health/route.ts','''export async function GET(){return Response.json({status:"ok"},{headers:{"Cache-Control":"no-store"}})}
''')
put(web,'deploy/compose.yaml','''name: afrigoals-ecommerce
services:
  api:
    build: ../../Afrigoals_ecommerce_api
    restart: unless-stopped
    env_file: ${API_ENV_FILE:-./api.env}
    environment:
      APP_ENV: production
      PORT: 8080
      DB_DRIVER: postgres
    healthcheck:
      test: [CMD, wget, -q, -O, /dev/null, http://127.0.0.1:8080/health]
      interval: 15s
      timeout: 5s
      retries: 6
      start_period: 180s
  web:
    build:
      context: ..
      args:
        NEXT_PUBLIC_API_URL: https://${API_DOMAIN:?Set API_DOMAIN}
        NEXT_PUBLIC_GOOGLE_MAPS_API: ${NEXT_PUBLIC_GOOGLE_MAPS_API:-}
        NEXT_PUBLIC_GOOGLE_MAP_ID: ${NEXT_PUBLIC_GOOGLE_MAP_ID:-}
        NEXT_PUBLIC_GOOGLE_CLIENT_ID: ${NEXT_PUBLIC_GOOGLE_CLIENT_ID:-}
        NEXT_PUBLIC_MOBILE_MONEY_ENABLED: 'true'
    restart: unless-stopped
    env_file: ${WEB_ENV_FILE:-./web.env}
    environment:
      AFRIGOALS_API_URL: http://api:8080
      NODE_ENV: production
    depends_on:
      api: {condition: service_healthy}
    healthcheck:
      test: [CMD, node, -e, "fetch('http://127.0.0.1:3000/api/health').then(r=>process.exit(r.ok?0:1)).catch(()=>process.exit(1))"]
      interval: 15s
      timeout: 5s
      retries: 6
      start_period: 60s
  caddy:
    image: caddy:2-alpine
    restart: unless-stopped
    ports: ['80:80', '443:443', '443:443/udp']
    environment:
      STORE_DOMAIN: ${STORE_DOMAIN:?Set STORE_DOMAIN}
      API_DOMAIN: ${API_DOMAIN:?Set API_DOMAIN}
      ACME_EMAIL: ${ACME_EMAIL:?Set ACME_EMAIL}
    volumes:
      - ./Caddyfile:/etc/caddy/Caddyfile:ro
      - caddy_data:/data
      - caddy_config:/config
    depends_on:
      web: {condition: service_healthy}
volumes:
  caddy_data:
  caddy_config:
''')
put(web,'deploy/Caddyfile','''{
    email {$ACME_EMAIL}
}
{$STORE_DOMAIN} {
    encode zstd gzip
    reverse_proxy web:3000
}
{$API_DOMAIN} {
    encode zstd gzip
    reverse_proxy api:8080
}
''')
put(web,'deploy/.env.example','''STORE_DOMAIN=shop.example.com
API_DOMAIN=api.example.com
ACME_EMAIL=admin@example.com
API_ENV_FILE=./api.env
WEB_ENV_FILE=./web.env
NEXT_PUBLIC_GOOGLE_MAPS_API=
NEXT_PUBLIC_GOOGLE_MAP_ID=
NEXT_PUBLIC_GOOGLE_CLIENT_ID=
''')
put(web,'deploy/web.env.example','''AI_GATEWAY_API_KEY=
SHOPPING_AI_MODEL=anthropic/claude-sonnet-4.5
''')
put(web,'deploy/README.md','''# Contabo deployment

The actual server hostname, SSH user and domains must be supplied by the owner.
No deployment is performed by preparing these files.

## Server layout

Place the repositories side-by-side:

```
/opt/afrigoals/Afrigoals_ecommerce
/opt/afrigoals/Afrigoals_ecommerce_api
```

Install Docker Engine with Compose on the server. Point storefront and API DNS
records at that server and permit inbound TCP 80/443 and optional UDP 443.

## Configuration

In the frontend `deploy` directory, copy `.env.example` to `.env`,
`web.env.example` to `web.env`, and the API's `.env.example` to `api.env`.
Set real domains, database DSN, R2 credentials, IOTEC merchant credentials,
email sender settings, strong JWT secret and 2FA pepper. Do not commit these files.
`BASE_URL` is the public HTTPS API origin; `CORS_ALLOW_ORIGINS` is the storefront
HTTPS origin. The API remains private to the Docker network behind Caddy.

Google browser keys and Google sign-in client IDs are build arguments; they must
be supplied before building. The Google Routes server key belongs only in
`api.env`. Configure OAuth's permitted origins for the final storefront domain.

The local testing delivery mode is flat UGX 300. Review Admin > Delivery pricing
and choose production rates before making the store publicly available.

## Release checks

Run the complete frontend/backend suites and isolated release database tests.
Verify password-reset email delivery, Google login, map selection and an owner-
approved IOTEC test payment on the target environment. Automated regression tests
do not prove external email/payment providers are configured correctly.
Back up the database before starting an updated API; startup adds the vendor and
delivery-settings fields/tables through the existing AutoMigrate mechanism.

## Start

```
docker compose --env-file .env -f compose.yaml build
docker compose --env-file .env -f compose.yaml up -d --wait
docker compose --env-file .env -f compose.yaml logs --tail=100 api web caddy
```

Check `https://API_DOMAIN/health` and `https://STORE_DOMAIN/api/health`, then
verify cross-account order access is denied and rider payouts match commission.
Keep the prior images available for rollback. Roll application images back
without dropping database tables or deleting customer/payment records.
''')
p=web/'.gitignore';s=p.read_text(encoding='utf-8')+'\n# Local deployment credentials\ndeploy/api.env\ndeploy/web.env\n';put(web,'.gitignore',s)
put(api,'services/production.go','''package services
import("errors";"strings")
func ValidateProductionConfig(c Config)error{
 if len(c.JWTSecret)<32||strings.Contains(c.JWTSecret,"change-me"){return errors.New("production requires a unique JWT_SECRET of at least 32 characters")}
 if len(c.TwoFAPepper)<32||strings.HasPrefix(c.TwoFAPepper,"dev-"){return errors.New("production requires a unique TWO_FA_PEPPER of at least 32 characters")}
 if c.DBDriver!="postgres"{return errors.New("production requires Postgres")}
 for _,password:=range []string{c.SeedAdminPassword,c.SeedUserPassword}{if password!=""&&len(password)<12{return errors.New("production seed passwords must contain at least 12 characters")}}
 return nil
}
''')
p=api/'main.go';s=p.read_text(encoding='utf-8').replace('"log"','"log"\n"os"',1).replace('cfg := services.LoadConfig()','cfg := services.LoadConfig()\n if os.Getenv("APP_ENV")=="production" { if err:=services.ValidateProductionConfig(cfg);err!=nil{log.Fatal(err)} }',1);put(api,'main.go',s)
put(api,'services/production_test.go','''package services
import"testing"
func TestProductionConfigRejectsDevelopmentCredentials(t *testing.T){
 if ValidateProductionConfig(Config{JWTSecret:"dev-secret-change-me"})==nil{t.Fatal("development credentials accepted")}
 valid:=Config{JWTSecret:"production-only-unique-secret-32-characters",TwoFAPepper:"production-only-unique-pepper-32-characters",DBDriver:"postgres"}
 if err:=ValidateProductionConfig(valid);err!=nil{t.Fatal(err)}
}
''')
print('Prepared Contabo Compose/TLS deployment and production credential validation.')
