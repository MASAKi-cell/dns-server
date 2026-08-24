package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/MASAKi-cell/dns/client"
	"github.com/MASAKi-cell/dns/message"
)

const version = "0.1"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintf(os.Stderr, "selfdig: %v\n", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	server := "8.8.8.8:53"
	var name string
	typ := message.TypeA

	for _, arg := range args {
		switch {
		case strings.HasPrefix(arg, "@"):
			server = strings.TrimPrefix(arg, "@")
			if !strings.Contains(server, ":") {
				server = server + ":53"
			}
		case isType(arg):
			typ = parseType(arg)
		default:
			name = arg
		}
	}

	if name == "" {
		return fmt.Errorf("usage: selfdig [@server] <name> [type]")
	}

	// ヘッダー出力
	fmt.Printf("; <<>> selfdig %s <<>> @%s %s %s\n", version, server, name, typ)
	fmt.Println()

	// DNS クエリ実行
	c := client.NewClient(
		client.WithServers(server),
		client.WithTimeout(5*time.Second),
		client.WithMaxRetries(2),
	)

	start := time.Now()
	resp, err := c.Query(context.Background(), name, typ)
	elapsed := time.Since(start)

	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	// QUESTION SECTION
	fmt.Println(";; QUESTION SECTION:")
	for _, q := range resp.Questions {
		fmt.Printf(";%-23s\t%s\t%s\n", q.Name, q.Class, q.Type)
	}
	fmt.Println()

	// ANSWER SECTION
	if len(resp.Answers) > 0 {
		fmt.Println(";; ANSWER SECTION:")
		for _, rr := range resp.Answers {
			fmt.Printf("%s\t%d\t%s\t%s\t%s\n",
				rr.Name, rr.TTL, rr.Class, rr.Type, formatRData(rr.RData))
		}
		fmt.Println()
	}

	// AUTHORITY SECTION
	if len(resp.Authorities) > 0 {
		fmt.Println(";; AUTHORITY SECTION:")
		for _, rr := range resp.Authorities {
			fmt.Printf("%s\t%d\t%s\t%s\t%s\n",
				rr.Name, rr.TTL, rr.Class, rr.Type, formatRData(rr.RData))
		}
		fmt.Println()
	}

	// ADDITIONAL SECTION
	if len(resp.Additionals) > 0 {
		fmt.Println(";; ADDITIONAL SECTION:")
		for _, rr := range resp.Additionals {
			fmt.Printf("%s\t%d\t%s\t%s\t%s\n",
				rr.Name, rr.TTL, rr.Class, rr.Type, formatRData(rr.RData))
		}
		fmt.Println()
	}

	// フッター
	fmt.Printf(";; Query time: %d msec\n", elapsed.Milliseconds())
	fmt.Printf(";; SERVER: %s\n", server)
	fmt.Printf(";; MSG SIZE rcvd: %d\n", estimateSize(resp))

	return nil
}

func isType(s string) bool {
	switch strings.ToUpper(s) {
	case "A", "NS", "CNAME", "SOA", "MX", "TXT", "AAAA":
		return true
	}
	return false
}

func parseType(s string) message.Type {
	switch strings.ToUpper(s) {
	case "A":
		return message.TypeA
	case "NS":
		return message.TypeNS
	case "CNAME":
		return message.TypeCNAME
	case "SOA":
		return message.TypeSOA
	case "MX":
		return message.TypeMX
	case "TXT":
		return message.TypeTXT
	case "AAAA":
		return message.TypeAAAA
	default:
		return message.TypeA
	}
}

func formatRData(rdata message.RData) string {
	if rdata == nil {
		return ""
	}
	switch v := rdata.(type) {
	case message.AData:
		return fmt.Sprintf("%d.%d.%d.%d", v.Address[0], v.Address[1], v.Address[2], v.Address[3])
	case message.AAAAData:
		return formatIPv6(v.Address)
	case message.NSData:
		return string(v.NSDName)
	case message.CNAMEData:
		return string(v.CName)
	case message.MXData:
		return fmt.Sprintf("%d %s", v.Preference, v.Exchange)
	case message.TXTData:
		return strings.Join(v.Txt, " ")
	case message.SOAData:
		return fmt.Sprintf("%s %s %d %d %d %d %d",
			v.MName, v.RName, v.Serial, v.Refresh, v.Retry, v.Expire, v.Minimum)
	default:
		return fmt.Sprintf("%v", rdata)
	}
}

func formatIPv6(addr [16]byte) string {
	return fmt.Sprintf("%x:%x:%x:%x:%x:%x:%x:%x",
		uint16(addr[0])<<8|uint16(addr[1]),
		uint16(addr[2])<<8|uint16(addr[3]),
		uint16(addr[4])<<8|uint16(addr[5]),
		uint16(addr[6])<<8|uint16(addr[7]),
		uint16(addr[8])<<8|uint16(addr[9]),
		uint16(addr[10])<<8|uint16(addr[11]),
		uint16(addr[12])<<8|uint16(addr[13]),
		uint16(addr[14])<<8|uint16(addr[15]))
}

func estimateSize(msg *message.Message) int {
	data, err := msg.Marshal()
	if err != nil {
		return 0
	}
	return len(data)
}
