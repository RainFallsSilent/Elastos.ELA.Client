package wallet

import (
	"os"
	"fmt"
	"bufio"
	"bytes"
	"errors"
	"strings"
	"strconv"
	"io/ioutil"

	tx "github.com/elastos/Elastos.ELA.Client/core/transaction"
	. "github.com/elastos/Elastos.ELA.Client/common"
	"github.com/elastos/Elastos.ELA.Client/common/log"
	"github.com/elastos/Elastos.ELA.Client/rpc"
	walt "github.com/elastos/Elastos.ELA.Client/wallet"

	"github.com/urfave/cli"
)

func createTransaction(c *cli.Context, wallet walt.Wallet) error {

	feeStr := c.String("fee")
	if feeStr == "" {
		return errors.New("use --fee to specify transfer fee")
	}

	fee, err := StringToFixed64(feeStr)
	if err != nil {
		return errors.New("invalid transaction fee")
	}

	from := c.String("from")
	if from == "" {
		from, err = selectAddress(wallet)
		if err != nil {
			return err
		}
	}

	multiOutput := c.String("file")
	if multiOutput != "" {
		return createMultiOutputTransaction(c, wallet, multiOutput, from, fee)
	}

	to := c.String("to")
	if to == "" {
		return errors.New("use --to to specify receiver address")
	}

	amountStr := c.String("amount")
	if amountStr == "" {
		return errors.New("use --amount to specify transfer amount")
	}

	amount, err := StringToFixed64(amountStr)
	if err != nil {
		return errors.New("invalid transaction amount")
	}

	lockStr := c.String("lock")
	var txn *tx.Transaction
	if lockStr == "" {
		txn, err = wallet.CreateTransaction(from, to, amount, fee)
		if err != nil {
			return errors.New("create transaction failed: " + err.Error())
		}
	} else {
		lock, err := strconv.ParseUint(lockStr, 10, 32)
		if err != nil {
			return errors.New("invalid lock height")
		}
		txn, err = wallet.CreateLockedTransaction(from, to, amount, fee, uint32(lock))
		if err != nil {
			return errors.New("create transaction failed: " + err.Error())
		}
	}

	output(0, 0, txn)

	return nil
}

func createMultiOutputTransaction(c *cli.Context, wallet walt.Wallet, path, from string, fee *Fixed64) error {
	if _, err := os.Stat(path); err != nil {
		return errors.New("invalid multi output file path")
	}
	file, err := os.OpenFile(path, os.O_RDONLY, 0666)
	if err != nil {
		return errors.New("open multi output file failed")
	}

	scanner := bufio.NewScanner(file)
	var multiOutput []*walt.Output
	for scanner.Scan() {
		columns := strings.Split(scanner.Text(), ",")
		if len(columns) < 2 {
			return errors.New(fmt.Sprint("invalid multi output line:", columns))
		}
		amountStr := strings.TrimSpace(columns[1])
		amount, err := StringToFixed64(amountStr)
		if err != nil {
			return errors.New("invalid multi output transaction amount: " + amountStr)
		}
		address := strings.TrimSpace(columns[0])
		multiOutput = append(multiOutput, &walt.Output{address, amount})
		log.Trace("Multi output address:", address, ", amount:", amount.String())
	}

	lockStr := c.String("lock")
	var txn *tx.Transaction
	if lockStr == "" {
		txn, err = wallet.CreateMultiOutputTransaction(from, fee, multiOutput...)
		if err != nil {
			return errors.New("create multi output transaction failed: " + err.Error())
		}
	} else {
		lock, err := strconv.ParseUint(lockStr, 10, 32)
		if err != nil {
			return errors.New("invalid lock height")
		}
		txn, err = wallet.CreateLockedMultiOutputTransaction(from, fee, uint32(lock), multiOutput...)
		if err != nil {
			return errors.New("create multi output transaction failed: " + err.Error())
		}
	}

	output(0, 0, txn)

	return nil
}

func signTransaction(name string, password []byte, context *cli.Context, wallet walt.Wallet) error {
	defer ClearBytes(password, len(password))

	content, err := getTransactionContent(context)
	if err != nil {
		return err
	}
	rawData, err := HexStringToBytes(content)
	if err != nil {
		return errors.New("decode transaction content failed")
	}

	var txn tx.Transaction
	err = txn.Deserialize(bytes.NewReader(rawData))
	if err != nil {
		return errors.New("deserialize transaction failed")
	}

	haveSign, needSign, err := txn.GetSignStatus()
	if haveSign == needSign {
		return errors.New("transaction was fully signed, no need more sign")
	}

	_, err = wallet.Sign(name, getPassword(password, false), &txn)
	if err != nil {
		return err
	}

	haveSign, needSign, _ = txn.GetSignStatus()
	fmt.Println("[", haveSign, "/", needSign, "] Transaction successfully signed")

	output(haveSign, needSign, &txn)

	return nil
}

func sendTransaction(context *cli.Context) error {
	content, err := getTransactionContent(context)
	if err != nil {
		return err
	}

	result, err := rpc.CallAndUnmarshal("sendrawtransaction", content)
	if err != nil {
		return err
	}
	fmt.Println(result.(string))
	return nil
}

func getTransactionContent(context *cli.Context) (string, error) {

	// If parameter with file path is not empty, read content from file
	if filePath := strings.TrimSpace(context.String("file")); filePath != "" {

		if _, err := os.Stat(filePath); err != nil {
			return "", errors.New("invalid transaction file path")
		}
		file, err := os.OpenFile(filePath, os.O_RDONLY, 0666)
		if err != nil {
			return "", errors.New("open transaction file failed")
		}
		rawData, err := ioutil.ReadAll(file)
		if err != nil {
			return "", errors.New("read transaction file failed")
		}

		content := strings.TrimSpace(string(rawData))
		// File content can not by empty
		if content == "" {
			return "", errors.New("transaction file is empty")
		}
		return content, nil
	}

	content := strings.TrimSpace(context.String("hex"))
	// Hex string content can not be empty
	if content == "" {
		return "", errors.New("transaction hex string is empty")
	}

	return content, nil
}

func output(haveSign, needSign int, txn *tx.Transaction) error {
	// Serialise transaction content
	buf := new(bytes.Buffer)
	txn.Serialize(buf)
	content := BytesToHexString(buf.Bytes())

	// Print transaction hex string content to console
	fmt.Println(content)

	// Output to file
	fileName := "to_be_signed" // Create transaction file name

	if haveSign == 0 {
		//	Transaction created do nothing
	} else if needSign > haveSign {
		fileName = fmt.Sprint(fileName, "_", haveSign, "_of_", needSign)
	} else if needSign == haveSign {
		fileName = "ready_to_send"
	}
	fileName = fileName + ".txn"

	file, err := os.OpenFile(fileName, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0666)
	if err != nil {
		return err
	}

	_, err = file.Write([]byte(content))
	if err != nil {
		return err
	}

	// Print output file to console
	fmt.Println("File: ", fileName)

	return nil
}
