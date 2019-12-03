module gitlab.com/arcanecrypto/teslacoil

go 1.13

// until https://github.com/btcsuite/btcd/pull/1500 is merged
replace github.com/btcsuite/btcd => github.com/arcanecryptoas/btcd v0.20.1-beta.0.20191126133409-e9546acdb9a0

require (
	github.com/araddon/dateparse v0.0.0-20190622164848-0fb0a474d195
	github.com/brianvoe/gofakeit v3.18.0+incompatible
	github.com/btcsuite/btcd v0.20.1-beta
	github.com/btcsuite/btcutil v0.0.0-20190425235716-9e5f4b9a998d
	github.com/dchest/authcookie v0.0.0-20190824115100-f900d2294c8e // indirect
	github.com/dchest/passwordreset v0.0.0-20190826080013-4518b1f41006
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gin-contrib/cors v1.3.0
	github.com/gin-gonic/gin v1.5.0
	github.com/go-playground/universal-translator v0.17.0 // indirect
	github.com/golang-migrate/migrate/v4 v4.5.0
	github.com/google/go-cmp v0.3.1
	github.com/gorilla/mux v1.7.3 // indirect
	github.com/iancoleman/strcase v0.0.0-20190422225806-e506e3ef7365
	github.com/jmoiron/sqlx v1.2.0
	github.com/json-iterator/go v1.1.8 // indirect
	github.com/leodido/go-urn v1.2.0 // indirect
	github.com/lib/pq v1.2.0
	github.com/lightninglabs/gozmq v0.0.0-20191113021534-d20a764486bf
	github.com/lightningnetwork/lnd v0.8.1-beta
	github.com/mattn/go-isatty v0.0.10 // indirect
	github.com/nbutton23/zxcvbn-go v0.0.0-20180912185939-ae427f1e4c1d
	github.com/pkg/errors v0.8.1
	github.com/pquerna/otp v1.2.0
	github.com/satori/go.uuid v1.2.0
	github.com/sendgrid/rest v2.4.1+incompatible
	github.com/sendgrid/sendgrid-go v3.5.0+incompatible
	github.com/sirupsen/logrus v1.4.2
	github.com/stretchr/testify v1.4.0
	github.com/urfave/cli v1.22.1
	github.com/ztrue/shutdown v0.1.1
	golang.org/x/crypto v0.0.0-20190829043050-9756ffdc2472
	golang.org/x/net v0.0.0-20190827160401-ba9fcec4b297 // indirect
	golang.org/x/sys v0.0.0-20191128015809-6d18c012aee9 // indirect
	google.golang.org/grpc v1.20.1
	gopkg.in/go-playground/validator.v9 v9.30.2
	gopkg.in/macaroon.v2 v2.1.0
	gopkg.in/yaml.v2 v2.2.7 // indirect
)
