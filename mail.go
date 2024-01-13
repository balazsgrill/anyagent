package main

import (
	"log"
	"time"

	"github.com/anyproto/anytype-heart/core/block/import/markdown/anymark"
	"github.com/anyproto/anytype-heart/pb"
	"github.com/anyproto/anytype-heart/pkg/lib/bundle"
	"github.com/anyproto/anytype-heart/pkg/lib/pb/model"
	"github.com/anyproto/anytype-heart/util/pbtypes"
	"github.com/balazsgrill/extractld"
	"github.com/gogo/protobuf/types"
	"github.com/google/uuid"
)

func MailSnapshot(email extractld.Mail) (*pb.RpcObjectImportRequestSnapshot, error) {
	blocks, _, err := anymark.HTMLToBlocks([]byte(email.Body()))
	if err != nil {
		return nil, err
	}

	return &pb.RpcObjectImportRequestSnapshot{
		Id: uuid.NewMD5(uuid.Nil, []byte(email.Source())).String(),
		Snapshot: &model.SmartBlockSnapshotBase{
			Blocks: blocks,
			Details: &types.Struct{
				Fields: map[string]*types.Value{
					bundle.RelationKeyName.String():        pbtypes.String(email.Topic()),
					bundle.RelationKeySource.String():      pbtypes.String(email.Source()),
					bundle.RelationKeyType.String():        pbtypes.String(bundle.TypeKeyPage.String()),
					bundle.RelationKeyCreatedDate.String(): pbtypes.Int64(email.SentTime().Unix()),
				},
			},
			OriginalCreatedTimestamp: email.SentTime().Unix(),
		},
	}, nil
}

func GetMail(provider extractld.MailProvider, start time.Time, end time.Time) ([]*pb.RpcObjectImportRequestSnapshot, error) {
	mails, err := provider.MailByDate(start, end)
	if err != nil {
		return nil, err
	}
	result := make([]*pb.RpcObjectImportRequestSnapshot, 0, len(mails))
	for _, mail := range mails {
		if mail.IsFlagged() {
			ss, err := MailSnapshot(mail)
			if err != nil {
				log.Println(err)
			} else {
				result = append(result, ss)
			}
		}
	}
	return result, nil
}

type MailProcessor struct {
	extractld.MailProvider
	lastTime time.Time
}

func (mp *MailProcessor) Do(now time.Time) []*pb.RpcObjectImportRequestSnapshot {
	mail, err := GetMail(mp.MailProvider, mp.lastTime, now)
	if err != nil {
		// Retry next time, don't progress last successful time
		return nil
	}
	mp.lastTime = now
	return mail
}
