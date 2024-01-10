package main

import (
	"context"
	"flag"
	"log"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/anytype-heart/core/application"
	"github.com/anyproto/anytype-heart/core/block"
	"github.com/anyproto/anytype-heart/core/block/object/objectcreator"
	"github.com/anyproto/anytype-heart/core/domain"
	"github.com/anyproto/anytype-heart/core/event"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/database"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
	"github.com/anyproto/anytype-heart/space"
	"github.com/anyproto/anytype-heart/util/pbtypes"
	"github.com/gogo/protobuf/types"

	_ "github.com/balazsgrill/extractld"
	_ "github.com/balazsgrill/oauthenticator"
)

func main() {
	path := flag.String("p", "", "storage path")
	mnemonic := flag.String("m", "", "Secret mnemonic")
	accountid := flag.String("a", "", "Account ID")
	flag.Parse()

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
	/*log.Println("Recovering account")
	err = instance.AccountRecover()
	if err != nil {
		panic(err)
	}*/

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

	objectstores := app.MustComponent[objectstore.ObjectStore](instance.GetApp())

	objectstores.Query(database.Query{
		Filters: []*model.BlockContentDataviewFilter{
			{
				RelationKey: bundle.RelationKeySource.String(),
				Condition:   model.BlockContentDataviewFilter_Equal,
				Value:       pbtypes.String(""),
			},
		},
	})

	blockservice := app.MustComponent[*block.Service](instance.GetApp())
	objectcreators := app.MustComponent[objectcreator.Service](instance.GetApp())
	templateID := "mycustomobject"
	smartblock, err := blockservice.GetObjectByFullID(context.Background(), domain.FullID{SpaceID: sp.Id(), ObjectID: templateID})
	if err != nil {
		createdobjectid, _, err := objectcreators.CreateObject(context.Background(), sp.Id(), objectcreator.CreateObjectRequest{
			ObjectTypeKey: bundle.TypeKeyNote,
			Details: &types.Struct{
				Fields: map[string]*types.Value{
					bundle.RelationKeyDescription.String(): pbtypes.String("Someting"),
				},
			},
		})
		if err != nil {
			panic(err)
		}
		log.Printf("created ID: %s", createdobjectid)
	} else {
		log.Printf("ID: %s", smartblock.Id())
	}

	defer instance.Stop()
}
