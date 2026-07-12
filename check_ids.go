//go:build ignore

package main

import (
	"fmt"
	"github.com/go-mclib/data/pkg/data/packet_ids"
)

func main() {
	fmt.Printf("S2CLoginID: 0x%X\n", packet_ids.S2CLoginID)
	fmt.Printf("S2CRespawnID: 0x%X\n", packet_ids.S2CRespawnID)
	fmt.Printf("S2CSetHealthID: 0x%X\n", packet_ids.S2CSetHealthID)
}
