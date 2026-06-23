package main

import (
	"bytes"
	"io"
	"net"
	"testing"

	"github.com/anantadwi13/protohackers/11-pest-control/proto"
	"github.com/anantadwi13/protohackers/11-pest-control/util"
	"github.com/stretchr/testify/assert"
)

func TestServer(t *testing.T) {
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		return
	}
	defer conn.Close()

	msgHello := &proto.MessageHello{
		Protocol: "pestcontrol",
		Version:  1,
	}
	_, err = msgHello.Marshal(conn)
	assert.NoError(t, err)

	_, _, err = msgHello.Unmarshal(conn)
	assert.NoError(t, err)

	msgSiteVisit := &proto.MessageSiteVisit{
		Site: 12345,
		Populations: *proto.NewArray(
			&proto.SiteVisitPopulation{
				Species: "cat",
				Count:   15,
			},
			&proto.SiteVisitPopulation{
				Species: "cat",
				Count:   10,
			},
		),
	}
	_, err = msgSiteVisit.Marshal(conn)
	assert.NoError(t, err)

	buf := util.GetBytes(2048)[:1]
	defer util.PutBytes(buf)

	n, err := io.ReadFull(conn, buf)
	assert.NoError(t, err)

	t.Log("received", buf[:n])

	msgError := &proto.MessageError{}
	assert.Equal(t, byte(msgError.Id()), buf[0])

	_, _, err = msgError.Unmarshal(io.MultiReader(bytes.NewReader(buf[:n]), conn))
	assert.NoError(t, err)

	t.Logf("msgError: %v", msgError)

	//t.Log("received:", string(buf[:n]))
}
