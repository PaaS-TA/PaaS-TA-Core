package dnsserver

import (
	"log"
	"net"

	miekgdns "github.com/miekg/dns"
)

type Server struct {
	tcpServer            *miekgdns.Server
	udpServer            *miekgdns.Server
	records              map[string][]miekgdns.RR
	truncatedDomainNames map[string]bool
}

func NewServer() Server {
	server := Server{
		records:              make(map[string][]miekgdns.RR),
		truncatedDomainNames: make(map[string]bool),
	}

	server.udpServer = &miekgdns.Server{
		Addr:    ":53",
		Net:     "udp",
		Handler: miekgdns.HandlerFunc(server.handleUDPDNSRequest),
	}

	server.tcpServer = &miekgdns.Server{
		Addr:    ":53",
		Net:     "tcp",
		Handler: miekgdns.HandlerFunc(server.handleTCPDNSRequest),
	}

	return server
}

func (s Server) Start() {
	go func() {
		err := s.tcpServer.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		err := s.udpServer.ListenAndServe()
		if err != nil {
			log.Fatal(err)
		}
	}()
}

func (s Server) Stop() error {
	err := s.udpServer.Shutdown()
	if err != nil {
		log.Fatal(err)
	}

	err = s.tcpServer.Shutdown()
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

func (Server) URL() string {
	return "127.0.0.1:53"
}

func (s Server) SetTruncateDNS(domainName string) {
	s.truncatedDomainNames[domainName+"."] = true
}

func (s Server) RegisterARecord(domainName string, ipAddress net.IP) {
	s.registerRecord(domainName, &miekgdns.A{
		Hdr: s.header(domainName, miekgdns.TypeA),
		A:   ipAddress,
	})
}

func (s Server) RegisterAAAARecord(domainName string, ipAddress net.IP) {
	s.registerRecord(domainName, &miekgdns.AAAA{
		Hdr:  s.header(domainName, miekgdns.TypeAAAA),
		AAAA: ipAddress,
	})
}

func (s Server) RegisterMXRecord(domainName string, target string, preference uint16) {
	s.registerRecord(domainName, &miekgdns.MX{
		Hdr:        s.header(domainName, miekgdns.TypeMX),
		Mx:         target,
		Preference: preference,
	})
}

func (s Server) DeregisterAllRecords() {
	s.records = make(map[string][]miekgdns.RR)
}
func mustRR(s string) miekgdns.RR {
	r, err := miekgdns.NewRR(s)
	if err != nil {
		log.Fatal(err)
	}
	return r
}

func (s Server) handleTCPDNSRequest(responseWriter miekgdns.ResponseWriter, requestMessage *miekgdns.Msg) {
	responseMessage := new(miekgdns.Msg)
	responseMessage.SetReply(requestMessage)
	resourceRecords, recordExists := s.records[requestMessage.Question[0].Name]

	if recordExists {
		log.Printf("Found record: %s\n", requestMessage.Question[0].Name)
		responseMessage.Answer = make([]miekgdns.RR, len(resourceRecords))
		for i, resourceRecord := range resourceRecords {
			responseMessage.Answer[i] = resourceRecord
		}
	} else {
		log.Printf("No record found: %s\n", requestMessage.Question[0].Name)
	}

	log.Println("Response to DNS request: ", responseMessage)
	responseWriter.WriteMsg(responseMessage)
}

func (s Server) handleUDPDNSRequest(responseWriter miekgdns.ResponseWriter, requestMessage *miekgdns.Msg) {
	responseMessage := new(miekgdns.Msg)
	responseMessage.SetReply(requestMessage)
	resourceRecords, recordExists := s.records[requestMessage.Question[0].Name]

	if recordExists {
		log.Printf("Found record: %s\n", requestMessage.Question[0].Name)
		responseMessage.Answer = make([]miekgdns.RR, len(resourceRecords))
		for i, resourceRecord := range resourceRecords {
			responseMessage.Answer[i] = resourceRecord
		}

		responseMessage.MsgHdr.Truncated = s.truncatedDomainNames[requestMessage.Question[0].Name]

	} else {
		log.Printf("No record found: %s\n", requestMessage.Question[0].Name)
	}

	log.Println("Response to DNS request: ", responseMessage)
	responseWriter.WriteMsg(responseMessage)
}

func (s Server) registerRecord(domainName string, resourceRecord miekgdns.RR) {
	_, exists := s.records[domainName+"."]

	if !exists {
		s.records[domainName+"."] = []miekgdns.RR{}
	}

	s.records[domainName+"."] = append(s.records[domainName+"."], resourceRecord)
}

func (s Server) header(domainName string, resourceRecordType uint16) miekgdns.RR_Header {
	return miekgdns.RR_Header{
		Name:   domainName + ".",
		Rrtype: resourceRecordType,
		Class:  miekgdns.ClassINET,
		Ttl:    0,
	}
}
