package core

import (
	"fmt"
	"strings"
)

type CIDRPool struct {
	ipStart       string
	cidrMask      uint
	cidrMaskBlock uint
	pool          []string
}

func NewCIDRPool(ipStart string, cidrMask, cidrMaskBlock uint) CIDRPool {
	return CIDRPool{
		ipStart:       ipStart,
		cidrMask:      cidrMask,
		cidrMaskBlock: cidrMaskBlock,
		pool:          generateCIDRPool(ipStart, cidrMask, cidrMaskBlock),
	}
}

func (c *CIDRPool) Get(index int) (string, error) {
	if len(c.pool) <= index {
		return "", fmt.Errorf("cannot get cidr of index %d when pool size is size of %d", index, len(c.pool))
	}

	return c.pool[index], nil
}

func (c *CIDRPool) Last() string {
	return c.pool[len(c.pool)-1]
}

func generateCIDRPool(ipStart string, cidrMask, cidrMaskBlock uint) []string {
	pool := []string{}
	fullRange := 1 << (32 - cidrMask)
	blockSize := 1 << (32 - cidrMaskBlock)
	for i := 0; i < fullRange; i += blockSize {
		pool = append(pool, fmt.Sprintf("%s/%d", buildNewIP(ipStart, i), cidrMaskBlock))
	}
	return pool
}

func buildNewIP(ip string, lastPart int) string {
	parts := strings.Split(string(ip), ".")
	return fmt.Sprintf("%s.%s.%s.%d", parts[0], parts[1], parts[2], lastPart)
}
