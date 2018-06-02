package wallet

import (
	"bytes"
	"errors"
	"fmt"

	. "github.com/elastos/Elastos.ELA.Utility/common"
	"github.com/elastos/Elastos.ELA.Utility/crypto"

	"github.com/urfave/cli"
)

func CalculateGenesisAddress(c *cli.Context) error {
	genesisBlockHash := c.String("genesis")
	if genesisBlockHash == "" {
		return errors.New("use --genesis to input genesis block hash")
	}

	genesisBlockBytes, err := HexStringToBytes(genesisBlockHash)
	if err != nil {
		return errors.New("genesis block hash to bytes failed")
	}

	buf := new(bytes.Buffer)
	buf.WriteByte(byte(len(genesisBlockBytes)))
	buf.Write(genesisBlockBytes)
	buf.WriteByte(byte(CROSSCHAIN))

	genesisProgramHash, err := crypto.ToProgramHash(buf.Bytes())
	if err != nil {
		return errors.New("genesis block bytes to program hash faild")
	}

	genesisAddress, err := genesisProgramHash.ToAddress()
	if err != nil {
		return errors.New("genesis block hash to genesis address failed")
	}
	fmt.Println("genesis address: ", genesisAddress)

	return nil
}
