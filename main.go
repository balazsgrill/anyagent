package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/anytype-heart/core/application"
	"github.com/anyproto/anytype-heart/core/block/collection"
	"github.com/anyproto/anytype-heart/core/event"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/space"
)

func main() {
	path := flag.String("p", "", "storage path")
	mnemonic := flag.String("m", "", "Secret mnemonic")
	accountid := flag.String("a", "", "Account ID")

	idle := &idledetector{
		duration: time.Second,
	}
	instance := application.New()
	// MetricsSetParameter
	instance.SetClientVersion("win32cli", "0.0.0")
	instance.SetEventSender(event.NewCallbackSender(
		func(event *pb.Event) {
			idle.NotIdle()
		}))
	log.Println("WalletRecover")
	err := instance.WalletRecover(&pb.RpcWalletRecoverRequest{
		RootPath: *path,
		Mnemonic: *mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println("WalletCreateSession")
	token, err := instance.CreateSession(&pb.RpcWalletCreateSessionRequest{
		Mnemonic: *mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println(token)
	//ListenSessionEvents
	//log.Println("Recovering account")
	//err = instance.AccountRecover()

	log.Println("AccountSelect")
	_, err = instance.AccountSelect(context.Background(), &pb.RpcAccountSelectRequest{
		Id:                      *accountid,
		RootPath:                *path,
		DisableLocalNetworkSync: false,
	})
	if err != nil {
		panic(err)
	}
	// Wait sync to stabilize (no events for a second?)
	idle.Wait()

	sps := app.MustComponent[space.Service](instance.GetApp())
	sp, _ := sps.GetPersonalSpace(context.Background())
	log.Printf("Space: %s", sp.Id())

	collections := app.MustComponent[*collection.Service](instance.GetApp())
	log.Printf("%s: %s", collections.Name(), collections.CollectionType())

	defer instance.Stop()
}
