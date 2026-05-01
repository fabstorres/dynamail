# Dynamail
A dyanmic software interface for Gmail with AI

## Prerequisites
- Go 1.25+
- Google OAuth2.0 credientals (Web application)

## Setup
- Clone the repo: `git clone https://github.com/fabstorres/dynamail.git`
- Change directory to 'apps/api': `cd dynamail/apps/api`
- Install go deps: `go mod download`
- Get Google OAuth2.0 credientials. [more information here](https://developers.google.com/workspace/gmail/api/quickstart/go)
- Create `.env` in `dynamail/apps/api`
- Run project with `go run cmd/main.go`

## Enviroment Variables

```bash
# Check internal/config/config.go to see all variables
PORT=
SESSION_SECRET=
GOOGLE_OAUTH_CLIENT_ID=
GOOGLE_OAUTH_CLIENT_SECRET=
GOOGLE_OAUTH_REDIRECT_URL=
```

## LICENSE

This project is licensed under the MIT License. See [LICENSE](./LICENSE) for details.
