package client

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"golang.org/x/crypto/ssh/terminal"

	"github.com/1071496910/mysh/proto"
)

var (
	EnvKeyUid = "MYSH_UID"
	EnvKeyPs  = "MYSH_PS"
)

type GetString func() string
type SetString func(string)

func makeLoginFunc(client proto.SearchServiceClient, uidGetter, psGetter GetString, uidSetter, psSetter SetString) func() string {
	uidSetter("")
	psSetter("")
	return func() string {

		needLogin := false

		if psGetter() == "" || uidGetter() == "" {
			needLogin = true
		} else {
			if resp, err := client.Login(context.Background(), &proto.LoginRequest{
				Uid:      uidGetter(),
				Password: psGetter(),
			}); err == nil {
				return resp.Token
			}
			needLogin = true

		}

		for needLogin {

			reader := bufio.NewReader(os.Stdin)
			fmt.Print("Enter username: ")
			u, _, _ := reader.ReadLine()

			fmt.Print("Enter password: ")
			p, err := terminal.ReadPassword(int(os.Stdin.Fd()))
			if err != nil {
				panic(err)
			}
			fmt.Println()
			uidSetter(string(u))
			psSetter(string(p))

			resp, err := client.Login(context.Background(), &proto.LoginRequest{
				Uid:      string(u),
				Password: string(p),
			})
			if err != nil {
				fmt.Println(err)
				continue
			}
			return resp.Token
		}
		return ""
	}

}

func MakeVarLoginFunc(client proto.SearchServiceClient) func() string {
	uidCache := ""
	passwordCache := ""
	uidGetter := GetString(func() string { return uidCache })
	uidSetter := SetString(func(s string) { uidCache = s })
	psGetter := GetString(func() string { return passwordCache })
	psSetter := SetString(func(s string) { passwordCache = s })

	return makeLoginFunc(client, uidGetter, psGetter, uidSetter, psSetter)
}

func MakeEnvLoginFunc(client proto.SearchServiceClient) func() string {
	uidGetter := GetString(func() string { return os.Getenv(EnvKeyUid) })
	uidSetter := SetString(func(s string) { os.Setenv(EnvKeyUid, s) })
	psGetter := GetString(func() string { return os.Getenv(EnvKeyPs) })
	psSetter := SetString(func(s string) { os.Setenv(EnvKeyPs, s) })

	return makeLoginFunc(client, uidGetter, psGetter, uidSetter, psSetter)

}

//func MakeLoginFunc(client proto.SearchServiceClient) func() string {
//
//	passwordCache := ""
//	uidCache := ""
//
//	return func() string {
//
//		needLogin := false
//
//		if passwordCache == "" || uidCache == "" {
//			needLogin = true
//		} else {
//			if resp, err := client.Login(context.Background(), &proto.LoginRequest{
//				Uid:      uidCache,
//				Password: passwordCache,
//			}); err == nil {
//				return resp.Token
//			}
//			needLogin = true
//
//		}
//
//		for needLogin {
//
//			reader := bufio.NewReader(os.Stdin)
//			fmt.Print("Enter username: ")
//			u, _, _ := reader.ReadLine()
//
//			fmt.Print("Enter password: ")
//			p, err := terminal.ReadPassword(int(os.Stdin.Fd()))
//			if err != nil {
//				panic(err)
//			}
//			fmt.Println()
//			uidCache = string(u)
//			passwordCache = string(p)
//
//			resp, err := client.Login(context.Background(), &proto.LoginRequest{
//				Uid:      string(u),
//				Password: string(p),
//			})
//			if err != nil {
//				fmt.Println(err)
//				continue
//			}
//			return resp.Token
//		}
//		return ""
//	}
//}
