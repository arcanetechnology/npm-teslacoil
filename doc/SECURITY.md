# Security in Teslacoil

## Critical packages

Here are some packages we utilize for security critical features in Teslacoil. These should
be scrutinized, both on the code front (what's in the package) and for any vulnerabilities
discovered in them (and how to mitigate)

- [`jwt-go`](https://godoc.org/github.com/dgrijalva/jwt-go) - Golang implementation of JSON Web Tokens (JWT)
- [`passwordreset`](https://godoc.org/github.com/dchest/passwordreset) - Small and nice package for generating one-time tokens for resetting passwords. 
- [`otp`](https://github.com/pquerna/otp) - Package for interacting with 2FA codes