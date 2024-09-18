#Google Authenticator Golang package
This Golang package can be used to interact with the Google Authenticator mobile app for 2-factor-authentication.
This package can generate secrets, generate codes and validate codes the secret.

For a secret installation you have to make sure that used codes cannot be reused (replay-attack).

#Usage:
>      package main
>      
>      import (
>        "flag"
>        "fmt"
>        "googleAuthenticator"
>      )
>      
>      func createSecret(ga *googleAuthenticator.GAuth) string {
>        secret, err := ga.CreateSecret(16)
>        if err != nil {
>        return ""
>        }
>        return secret
>      }
>      
>      func getCode(ga *googleAuthenticator.GAuth, secret string) string {
>        code, err := ga.GetCode(secret)
>        if err != nil {
>        return "*"
>        }
>        return code
>      }
>      
>      func verifyCode(ga *googleAuthenticator.GAuth, secret, code string) bool {
>        // 1:30sec
>        ret, err := ga.VerifyCode(secret, code, 1)
>        if err != nil {
>          return false
>        }
>        return ret
>      }
>       
>      func main() {
>        flag.Parse()
>        secret := flag.Arg(0)
>        ga := googleAuthenticator.NewGAuth()
>        fmt.Print(getCode(ga, secret))
>      }
