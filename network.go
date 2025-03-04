package main

import (
	"encoding/json"
	"fmt"
	"github.com/gcash/bchd/wire"
	"log"
	"os"
	"path"
	"strconv"
	"strings"
)

// JNetwork is the exported struct that is read from the network file
type JNetwork struct {
	Name            string
	Desc            string
	ID              string
	Port            uint16
	Pver            uint32
	DNSName         string
	NameServer      string
	Mbox            string
	TTL             uint32
	InitialIPs      []string
	Seeder1         string
	Seeder2         string
	Seeder3         string
	ServiceFilter   []string
	UserAgentFilter []string
}

func createNetFile() {
	// create a standard json template file that can be loaded into the app

	// create a struct to encode with json
	jnw := &JNetwork{
		ID:         "0xa0431619",
		Port:       56740,
		Pver:       70917,
		TTL:        600,
		DNSName:    "seeds.dogec.io",
		NameServer: "x9.seeds.dogec.io",
		Name:       "DogeCashNet",
		Mbox:       "james.s.dogec.io", // @ symbol replaced with period
		Desc:       "DogeCash Network Seeder",
		Seeder1:    "seeds.dogec.io",
		Seeder2:    "x9.seeds.dogec.io",
	}

	f, err := os.Create("dnsseeder.json")
	if err != nil {
		log.Printf("error creating template file: %v\n", err)
	}
	defer f.Close()

	j, jerr := json.MarshalIndent(jnw, "", " ")
	if jerr != nil {
		log.Printf("error parsing json: %v\n", err)
	}
	_, ferr := f.Write(j)
	if ferr != nil {
		log.Printf("error writing to template file: %v\n", err)
	}
}

func loadNetwork(fName string) (*dnsseeder, error) {
	nwFile, err := os.Open(fName)
	if err != nil {
		return nil, fmt.Errorf("Error reading network file: %v", err)
	}

	defer nwFile.Close()

	var jnw JNetwork

	jsonParser := json.NewDecoder(nwFile)
	if err = jsonParser.Decode(&jnw); err != nil {
		return nil, fmt.Errorf("Error decoding network file: %v", err)
	}

	return initNetwork(jnw)
}

func initNetwork(jnw JNetwork) (*dnsseeder, error) {

	if jnw.Port == 0 {
		return nil, fmt.Errorf("Invalid port supplied: %v", jnw.Port)

	}

	if jnw.DNSName == "" {
		return nil, fmt.Errorf("No DNS Hostname supplied")
	}

	// init the seeder
	seeder := &dnsseeder{}
	seeder.theList = make(map[string]*node)
	seeder.port = jnw.Port
	seeder.pver = jnw.Pver
	seeder.ttl = jnw.TTL
	seeder.name = jnw.Name
	seeder.desc = jnw.Desc
	seeder.dnsHost = jnw.DNSName
	seeder.nameServer = jnw.NameServer
	seeder.mbox = jnw.Mbox

	// conver the network magic number to a Uint32
	t1, err := strconv.ParseUint(jnw.ID, 0, 32)
	if err != nil {
		return nil, fmt.Errorf("Error converting Network Magic number: %v", err)
	}
	seeder.id = wire.BitcoinNet(t1)

	seeder.initialIPs = jnw.InitialIPs

	// load the seeder dns
	seeder.seeders = make([]string, 3)
	seeder.seeders[0] = jnw.Seeder1
	seeder.seeders[1] = jnw.Seeder2
	seeder.seeders[2] = jnw.Seeder3

	// Parse service flags
	var services []wire.ServiceFlag
	for _, service := range jnw.ServiceFilter {
		switch strings.ToLower(service) {
		case "nodenetwork":
			services = append(services, wire.SFNodeNetwork)
		case "nodegetutxo":
			services = append(services, wire.SFNodeGetUTXO)
		case "nodebloom":
			services = append(services, wire.SFNodeBloom)
		case "nodexthin":
			services = append(services, wire.SFNodeXthin)
		case "nodecf":
			services = append(services, wire.SFNodeCF)
		case "nodebitcoincash":
			services = append(services, wire.SFNodeBitcoinCash)
		}
	}
	seeder.serviceFilter = services
	seeder.userAgentFilter = jnw.UserAgentFilter

	// add some checks to the start & delay values to keep them sane
	seeder.maxStart = []uint32{20, 20, 20, 30}
	seeder.delay = []int64{210, 789, 234, 1876}
	seeder.maxSize = 100000

	// initialize the stats counters
	seeder.counts.NdStatus = make([]uint32, maxStatusTypes)
	seeder.counts.NdStarts = make([]uint32, maxStatusTypes)
	seeder.counts.DNSCounts = make([]uint32, maxDNSTypes)

	// some sanity checks on the loaded config options
	if seeder.ttl < 60 {
		seeder.ttl = 60
	}

	if dup, err := isDuplicateSeeder(seeder); dup {
		return nil, err
	}

	cacheFile, err := os.Open(path.Join(config.dataDir, fmt.Sprintf("%s.json", seeder.name)))
	if err == nil {
		defer cacheFile.Close()
		jsonParser := json.NewDecoder(cacheFile)
		if err = jsonParser.Decode(&seeder.theList); err != nil {
			log.Printf("Error decoding cache file: %v", err)
		}
		log.Printf("Loaded %d nodes from %s cache\n", len(seeder.theList), seeder.name)
	}

	return seeder, nil
}
