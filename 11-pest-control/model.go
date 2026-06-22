package _1_pest_control

import "github.com/anantadwi13/protohackers/11-pest-control/proto"

type SiteId proto.U32

type MessageHello struct {
	Protocol proto.String
	Version  proto.U32
}

type Error struct {
	Message proto.String
}

type Ok struct {
}
