package goridium

import (
	"bufio"
	"fmt"
	"go.bug.st/serial.v1"
	"log"
	"strconv"
	"strings"
	"time"
)

const (
	IridiumEpoch = 1399818235000 // May 11, 2014, at 14:23:55
)

type RockBlock struct {
	Path    string
	Port    serial.Port
	Scanner *bufio.Scanner
}

type SbdixReply struct {
	MoStatus int64
	Msn      int64
	MtStatus int64
	MtMsn    int64
	MtLength int64
	MtQueued int64
}

func NewRockBlock(path string) (rb *RockBlock, err error) {
	mode := &serial.Mode{
		BaudRate: 19200,
	}
	port, err := serial.Open(path, mode)
	if err != nil {
		return nil, err
	}

	rb = &RockBlock{
		Path:    path,
		Port:    port,
		Scanner: bufio.NewScanner(port),
	}

	return
}

func (rb *RockBlock) ReadLine() (r string, err error) {
	for rb.Scanner.Scan() {
		line := strings.TrimSpace(rb.Scanner.Text())
		if line != "" {
			log.Printf("# %s", line)
			return line, nil
		}
	}

	return
}

func (rb *RockBlock) Send(command string) (err error) {
	log.Printf("> %s", command)
	_, err = rb.Port.Write([]byte(command))
	return
}

func (rb *RockBlock) Close() (err error) {
	return rb.Port.Close()
}

func (rb *RockBlock) SendAndReadReply(command string) (reply string, err error) {
	err = rb.Send(command + "\r")
	if err != nil {
		return "", err
	}

	err = rb.Expect(command)
	if err != nil {
		return "", err
	}

	r, err := rb.ReadLine()
	if err != nil {
		return "", err
	}
	return r, nil
}

func (rb *RockBlock) Expect(expected string) (err error) {
	r, err := rb.ReadLine()
	if err != nil {
		return err
	}

	if r != expected {
		return fmt.Errorf("Unexpected reply (got %v, expected %v)",
			strings.Replace(r, "\r", "<cr>", -1),
			strings.Replace(expected, "\r", "<cr>", -1))
	}
	return
}

func (rb *RockBlock) GetSignalStrength() (s int64, err error) {
	reply, err := rb.SendAndReadReply("AT+CSQ")
	if err != nil {
		return -1, err
	}

	if strings.Index(reply, "+CSQ") < 0 || len(reply) != 6 {
		return -1, fmt.Errorf("Unexpected reply: %s", reply)
	}

	err = rb.Expect("OK")
	if err != nil {
		return -1, err
	}

	s, err = strconv.ParseInt(string(reply[5]), 10, 32)
	if err != nil {
		return -1, err
	}

	return
}

func (rb *RockBlock) GetNetworkTime() (time int64, err error) {
	reply, err := rb.SendAndReadReply("AT-MSSTM")
	if err != nil {
		return -1, err
	}

	err = rb.Expect("OK")
	if err != nil {
		return -1, err
	}

	if strings.Index(reply, "-MSSTM:") < 0 {
		return -1, fmt.Errorf("Unexpected reply: %s", reply)
	}

	time, err = strconv.ParseInt(string(reply[8:]), 16, 32)
	if err != nil {
		return -1, fmt.Errorf("No network time: %s", reply[8:])
	}

	time = (IridiumEpoch + (time * 90)) / 1000

	return
}

func (rb *RockBlock) GetSerialIdentifier() (serial string, err error) {
	reply, err := rb.SendAndReadReply("AT+GSN")
	if err != nil {
		return "", err
	}

	err = rb.Expect("OK")
	if err != nil {
		return "", err
	}

	return reply, nil
}

func (rb *RockBlock) ProcessMtMessage(mtMsn int64) (message string, err error) {
	command := "AT+SBDRB"
	err = rb.Send(command + "\r")
	if err != nil {
		return "", err
	}

	line, err := rb.ReadLine()
	if err != nil {
		return "", err
	}

	message = strings.TrimSpace(strings.Replace(line, command+"\r", "", 1))
	if len(message) > 4 {
		message = message[2:len(message)-2]
		log.Printf("%d '%s'", len(message), message)
	} else {
		message = ""
	}

	err = rb.Expect("OK")
	if err != nil {
		return "", fmt.Errorf("Expected OK: %v", err)
	}

	return
}

func (rb *RockBlock) QueueMessage(message string) (err error) {
	if len(message) > 340 {
		return fmt.Errorf("Message is too long, should be < 340 and is %d", len(message))
	}

	command := fmt.Sprintf("AT+SBDWB=%d", len(message))
	err = rb.Send(command + "\r")
	if err != nil {
		return err
	}

	err = rb.Expect(command)
	if err != nil {
		return err
	}

	err = rb.Expect("READY")
	if err != nil {
		return err
	}

	checksum := 0
	for _, c := range message {
		checksum += int(c)
	}

	rb.Send(message)
	rb.Send(fmt.Sprintf("%c", checksum>>8))
	rb.Send(fmt.Sprintf("%c", checksum&0xff))

	err = rb.Expect("0")
	if err != nil {
		return err
	}

	err = rb.Expect("OK")
	if err != nil {
		return err
	}

	return
}

func (rb *RockBlock) ClearMoBuffer() (err error) {
	_, err = rb.SendAndReadReply("AT+SBDD0")
	if err != nil {
		return err
	}

	err = rb.Expect("OK")
	if err != nil {
		return err
	}

	return
}

func ParseSbdixReply(reply string) (sr *SbdixReply, err error) {
	fields := strings.Split(strings.TrimSpace(reply[7:]), ", ")
	integers := make([]int64, 0)
	for _, field := range fields {
		i, err := strconv.ParseInt(field, 10, 32)
		if err != nil {
			return nil, err
		}

		integers = append(integers, i)

	}

	sr = &SbdixReply{
		MoStatus: integers[0],
		Msn:      integers[1],
		MtStatus: integers[2],
		MtMsn:    integers[3],
		MtLength: integers[4],
		MtQueued: integers[5],
	}

	return
}

func (rb *RockBlock) IsNetworkTimeValid() (err error) {
	reply, err := rb.SendAndReadReply("AT-MSSTM")
	if err != nil {
		return err
	}

	err = rb.Expect("OK")
	if err != nil {
		return err
	}

	// The length includes the prefix.
	if strings.Index(reply, "-MSSTM:") < 0 || len(string(reply)) != 16 {
		return fmt.Errorf("Unexpected reply: %s", reply)
	}

	return
}

func (rb *RockBlock) AttemptConnection() (err error) {
	timeAttempts := 20
	timeDelay := 1

	log.Printf("Attempting connection (attempts=%d, delay=%d)", timeAttempts, timeDelay)

	for {
		if timeAttempts == 0 {
			return fmt.Errorf("Unable to establish connection")
		}

		if rb.IsNetworkTimeValid() == nil {
			break
		}

		timeAttempts -= 1
		time.Sleep(time.Duration(timeDelay) * time.Second)
	}

	signalAttempts := 10
	signalDelay := 10
	signalThreshold := 2

	log.Printf("Waiting for signal of %d (attempts=%d, delay=%d)", signalThreshold, signalAttempts, signalDelay)

	for {
		signal, err := rb.GetSignalStrength()
		if err != nil {
			return err
		}

		if signalAttempts == 0 || signal < 0 {
			return fmt.Errorf("Unable to find required signal")
		}

		if int(signal) >= signalThreshold {
			return nil
		}

		signalAttempts -= 1
		time.Sleep(time.Duration(signalDelay) * time.Second)
	}

}

func (rb *RockBlock) AttemptSession() (incoming []string, err error) {
	attempts := 3

	incoming = make([]string, 0)

	log.Printf("Attempt session (attempts=%d)", attempts)

	for {
		if attempts == 0 {
			return incoming, fmt.Errorf("Unable to establish session")
		}

		attempts -= 1

		command := "AT+SBDIX"
		reply, err := rb.SendAndReadReply(command)
		if err != nil {
			return incoming, err
		}

		if strings.Index(reply, "+SBDIX:") >= 0 {
			sr, err := ParseSbdixReply(reply)
			if err != nil {
				return incoming, err
			}

			err = rb.Expect("OK")
			if err != nil {
				return incoming, err
			}

			success := false
			if sr.MoStatus <= 4 {
				rb.ClearMoBuffer()
				success = true
			}

			if sr.MtStatus == 1 && sr.MtLength > 0 {
				message, err := rb.ProcessMtMessage(sr.MtQueued)
				if err != nil {
					return incoming, err
				}
				if message != "" {
					incoming = append(incoming, message)
				}
			}
			if sr.MtQueued > 0 {
				rb.AttemptSession()
			}

			if success {
				return incoming, nil
			}
		}
	}

	return
}

func (rb *RockBlock) EnableEcho() (err error) {
	reply, err := rb.SendAndReadReply("ATE1")
	if err != nil {
		return err
	}
	if reply != "OK" {
		return fmt.Errorf("RockBlock enable echo failed")
	}
	return nil
}

func (rb *RockBlock) DisableFlowControl() (err error) {
	reply, err := rb.SendAndReadReply("AT&K0")
	if err != nil {
		return err
	}
	if reply != "OK" {
		return fmt.Errorf("RockBlock disable flow control failed")
	}
	return nil
}

func (rb *RockBlock) DisableRingAlerts() (err error) {
	reply, err := rb.SendAndReadReply("AT+SBDMTA=0")
	if err != nil {
		return err
	}
	if reply != "OK" {
		return fmt.Errorf("RockBlock ring alerts failed")
	}
	return nil
}

func (rb *RockBlock) Ping() (err error) {
	reply, err := rb.SendAndReadReply("AT")
	if err != nil {
		return err
	}
	if reply != "OK" {
		return fmt.Errorf("RockBlock ping failed")
	}
	return nil
}
