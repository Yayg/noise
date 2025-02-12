package skademlia

import (
	"fmt"
	"github.com/Yayg/noise"
	"github.com/Yayg/noise/payload"
	"github.com/stretchr/testify/assert"
	"testing"
)

var (
	publicKey = []byte("12345678901234567890123456789012")
	id        = NewID("address", publicKey, []byte{})
)

func TestPing(t *testing.T) {
	t.Parallel()
	p := Ping{}

	// good
	{
		msg, err := p.Read(payload.NewReader(id.Write()))
		assert.Nil(t, err)
		_, castOK := msg.(Ping)
		assert.True(t, castOK)
		assert.Truef(t, id.Equals(msg.(Ping).ID), "Expected equal %v vs %v", id, msg)
	}

	// bad
	{
		_, err := p.Read(payload.NewReader([]byte("bad")))
		assert.NotNil(t, err)
	}
}

func TestEvict(t *testing.T) {
	t.Parallel()
	e := Evict{}

	// evict doesn't implement read/write
	// so it looks the same as noise.EmptyMessage
	msg, err := e.Read(payload.NewReader(e.Write()))
	assert.Nil(t, err)
	_, castOK := msg.(noise.EmptyMessage)
	assert.True(t, castOK)
}

func TestLookupRequest(t *testing.T) {
	t.Parallel()
	lr := LookupRequest{}

	// good
	{
		msg, err := lr.Read(payload.NewReader(id.Write()))
		assert.Nil(t, err)
		_, castOK := msg.(LookupRequest)
		assert.True(t, castOK)
		assert.Truef(t, id.Equals(msg.(LookupRequest).ID), "Expected equal %v vs %v", id, msg)
	}

	// bad
	{
		_, err := lr.Read(payload.NewReader([]byte("bad")))
		assert.NotNil(t, err)
	}
}

func TestLookupResponse(t *testing.T) {
	t.Parallel()

	// bad
	{
		lr := LookupResponse{}
		_, err := lr.Read(payload.NewReader([]byte("bad")))
		assert.NotNil(t, err)
	}

	// normal cases
	testCases := []LookupResponse{
		{
			peers: []ID{}, // blank
		},
		{
			peers: []ID{id}, // 1 entry
		},
		{
			peers: []ID{id, id}, // 2 entries
		},
	}

	for i, lr := range testCases {
		t.Run(fmt.Sprintf("%d", i), func(t *testing.T) {
			wrote := lr.Write()
			assert.True(t, len(wrote) > 0, "bytes should not be empty")
			placeholder := LookupResponse{}
			msg, err := placeholder.Read(payload.NewReader(payload.NewWriter(wrote).Bytes()))
			assert.Nil(t, err)
			actual, ok := msg.(LookupResponse)
			assert.True(t, ok)
			assert.Equal(t, len(lr.peers), len(actual.peers))
			assert.EqualValues(t, lr, actual)
		})
	}
}
