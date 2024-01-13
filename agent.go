package main

import (
	"context"
	"log"
	"time"

	"github.com/anyproto/any-sync/app"
	"github.com/anyproto/anytype-heart/core/application"
	bimport "github.com/anyproto/anytype-heart/core/block/import"
	"github.com/anyproto/anytype-heart/core/block/process"
	"github.com/anyproto/anytype-heart/core/event"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/database"
	"github.com/anyproto/anytype-heart/pkg/lib/localstore/objectstore"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
	"github.com/anyproto/anytype-heart/space"
	"github.com/anyproto/anytype-heart/space/clientspace"
	"github.com/anyproto/anytype-heart/util/pbtypes"
)

type AgentConfig struct {
	Storagepath string
	Mnemonic    string
	Accountid   string
}

type agent struct {
	AgentConfig

	idle     *idledetector
	instance *application.Service

	spaceService       space.Service
	personalSpace      clientspace.Space
	importer           bimport.Importer
	objectstoreservice objectstore.ObjectStore
}

func New(config AgentConfig) *agent {
	return &agent{
		AgentConfig: config,
	}
}

func (a *agent) Init() {
	a.idle = &idledetector{
		duration: time.Second,
	}
	a.instance = application.New()
}

func (a *agent) Stop() {
	a.instance.Stop()
}

func (a *agent) ImportSnapshots(snapshots []*pb.RpcObjectImportRequestSnapshot) error {
	_, _, err := a.importer.Import(context.Background(), &pb.RpcObjectImportRequest{
		SpaceId:               a.personalSpace.Id(),
		UpdateExistingObjects: true,
		Type:                  model.Import_External,
		Mode:                  pb.RpcObjectImportRequest_IGNORE_ERRORS,
		NoProgress:            true,
		IsMigration:           false,
		IsNewSpace:            false,
		Snapshots:             snapshots,
	}, model.ObjectOrigin_none, process.NewNoOp())
	return err
}

func (a *agent) IsObjectExistBySource(source string) bool {
	records, total, _ := a.objectstoreservice.Query(database.Query{
		Filters: []*model.BlockContentDataviewFilter{
			{
				RelationKey: bundle.RelationKeySource.String(),
				Condition:   model.BlockContentDataviewFilter_Equal,
				Value:       pbtypes.String(source),
			},
		},
	})
	return total > 0 || len(records) > 0
}

func (a *agent) Start() {
	a.instance.SetClientVersion("win32cli", "0.0.0")
	a.instance.SetEventSender(event.NewCallbackSender(
		func(event *pb.Event) {
			a.idle.NotIdle()
		}))
	log.Println("WalletRecover")
	err := a.instance.WalletRecover(&pb.RpcWalletRecoverRequest{
		RootPath: a.Storagepath,
		Mnemonic: a.Mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println("WalletCreateSession")
	token, err := a.instance.CreateSession(&pb.RpcWalletCreateSessionRequest{
		Mnemonic: a.Mnemonic,
	})
	if err != nil {
		panic(err)
	}
	log.Println(token)

	log.Println("AccountSelect")
	_, err = a.instance.AccountSelect(context.Background(), &pb.RpcAccountSelectRequest{
		Id:                      a.Accountid,
		RootPath:                a.Storagepath,
		DisableLocalNetworkSync: false,
	})
	if err != nil {
		panic(err)
	}
	// Wait sync to stabilize (no events for a second?)
	a.idle.Wait()

	a.spaceService = app.MustComponent[space.Service](a.instance.GetApp())
	a.personalSpace, _ = a.spaceService.GetPersonalSpace(context.Background())

	a.importer = app.MustComponent[bimport.Importer](a.instance.GetApp())
	a.objectstoreservice = app.MustComponent[objectstore.ObjectStore](a.instance.GetApp())
}
