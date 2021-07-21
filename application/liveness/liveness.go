package liveness

import (
	"fmt"
	"net"
	"strconv"
	"time"
	"encoding/csv"
	"os"
	"os/exec"
)

type LivenessTester interface {
    PhantomIsLive(addr string, port uint16) (bool, error)
}

type CachedLivenessTester struct{
	ip_cache map[string]bool
}

type UncachedLivenessTester struct{
}


func (blt *CachedLivenessTester) Init(){
	blt.ip_cache = make(map[string]bool)
}

//limit should be left empty if scanning the whole internet, for local test only
//Call with goroutine
func (blt *CachedLivenessTester) Periodic_scan(){
	//For testing
	os.Create("block_list.txt")
	for{
		//_, err := exec.Command("sudo","zmap","-B",bandwidth,"-p",port,limit,"-o","result.csv").Output()
		_, err := exec.Command("zmap","-p","443","-O","csv","-f","saddr,classification","-P","4","--output-filter= (classification = rst || classification = synack)","-b","block_list.txt","-w","allow_list.txt","-o","result.csv").Output()
		if err != nil {
			fmt.Println("err")
			fmt.Println(err)
		}
		fmt.Println("1")
		//fmt.Println(output)
		f, err := os.Open("result.csv")
		if err != nil {
			fmt.Println("Unable to read input file", err)
			f.Close()
		}
		fmt.Println("2")
		csvReader := csv.NewReader(f)
		records, err := csvReader.ReadAll()
		if err != nil {
			fmt.Println("Unable to parse file as CSV", err)
		}
		
		//fmt.Println(records)
		f.Close()
		fmt.Println("3")
		//fmt.Println()
		f, err = os.OpenFile("block_list.txt", os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			fmt.Println("Unable to read blocklist file", err)
			f.Close()
		}
		fmt.Println("4")
		for _, ip := range records{
			//fmt.Println(ip[0])
			if ip[0] != "saddr"{
				if _, ok := blt.ip_cache[ip[0]]; !ok {
					blt.ip_cache[ip[0]] = true
					_, err := f.WriteString(ip[0]+"/32"+"\n")
					if err != nil {
						fmt.Println("Unable to write blocklist file", err)
						f.Close()
					}
				}
			}
		}
		f.Close()
		//fmt.Println(blt.ip_cache)
		//block_list.txt

		fmt.Println("Scanned once")
		//time.Sleep(time.Hour * 2)
		time.Sleep(time.Minute * 2)
	}
}

// PhantomIsLive - Test whether the phantom is live using
// 8 syns which returns syn-acks from 99% of sites within 1 second.
// see  ZMap: Fast Internet-wide Scanning  and Its Security Applications
// https://www.usenix.org/system/files/conference/usenixsecurity13/sec13-paper_durumeric.pdf
//
// return:	bool	true  - host is live
// 					false - host is not liev
//			error	reason decision was made
func (blt *CachedLivenessTester) PhantomIsLive(addr string, port uint16) (bool, error){
    // existing phantomIsLive() implementation
	if _, ok := blt.ip_cache[addr]; ok {
		return true, fmt.Errorf("cached live host")
	}
	isLive, err := phantomIsLive(net.JoinHostPort(addr, strconv.Itoa(int(port))))
	if isLive {
		blt.ip_cache[addr] = true
	}
	return isLive, err
}

func (blt *UncachedLivenessTester) PhantomIsLive(addr string, port uint16) (bool, error) {
	return phantomIsLive(net.JoinHostPort(addr, strconv.Itoa(int(port))))
}

func phantomIsLive(address string) (bool, error) {
	
	width := 4
	dialError := make(chan error, width)
	timeout := 750 * time.Millisecond

	testConnect := func() {
		conn, err := net.DialTimeout("tcp", address, timeout)
		if err != nil {
			dialError <- err
			return
		}
		conn.Close()
		dialError <- nil
	}

	for i := 0; i < width; i++ {
		go testConnect()
	}

	time.Sleep(timeout)

	// If any return errors or connect then return nil before deadline it is live
	select {
	case err := <-dialError:
		if e, ok := err.(net.Error); ok && e.Timeout() {
			return false, fmt.Errorf("Reached connection timeout")
		}
		if err != nil {
			return true, err
		}
		return true, fmt.Errorf("Phantom picked up the connection")
	default:
		return false, fmt.Errorf("Reached statistical timeout %v", timeout)
	}
}
