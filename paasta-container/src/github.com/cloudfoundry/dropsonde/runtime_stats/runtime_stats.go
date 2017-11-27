package runtime_stats

import (
	"log"
	"runtime"
	"time"
	"sort"
	"fmt"
	"strconv"
	"strings"
	"github.com/cloudfoundry/sonde-go/events"
	"github.com/gogo/protobuf/proto"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/net"
	"github.com/shirou/gopsutil/process"
)

type ProcessStatArray []ProcessStat
type ProcessStat struct {
	pid           int32
	ppid          int32
	startTime      string
	memUsage       uint64
	name          string
}

type EventEmitter interface {
	Emit(events.Event) error
}

type RuntimeStats struct {
	emitter  EventEmitter
	interval time.Duration
}

func NewRuntimeStats(emitter EventEmitter, interval time.Duration) *RuntimeStats {
	return &RuntimeStats{
		emitter:  emitter,
		interval: interval,
	}
}

func (rs *RuntimeStats) Run(stopChan <-chan struct{}) {
	ticker := time.NewTicker(rs.interval)
	defer ticker.Stop()
	for {
		fmt.Println("xxxxxxxx=====Container======xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx")
		rs.emit("numCPUS", float64(runtime.NumCPU()))
		rs.emit("numGoRoutines", float64(runtime.NumGoroutine()))
		rs.emitMemMetrics()

		//Add CPU Metrics
		rs.emitCpuMetrics()
		//Add Disk Metrics
		rs.emitDiskMetrics()
		//Add Network Metrics
		rs.emitNetworkMetrics()
		//Add Process Metrics
		rs.emitProcessMetrics()

		select {
		case <-ticker.C:
		case <-stopChan:
			return
		}
	}
}

func (rs *RuntimeStats) emitMemMetrics() {
	/*stats := new(runtime.MemStats)
	runtime.ReadMemStats(stats)

	rs.emit("memoryStats.numBytesAllocatedHeap", float64(stats.HeapAlloc))
	rs.emit("memoryStats.numBytesAllocatedStack", float64(stats.StackInuse))
	rs.emit("memoryStats.numBytesAllocated", float64(stats.Alloc))
	rs.emit("memoryStats.numMallocs", float64(stats.Mallocs))
	rs.emit("memoryStats.numFrees", float64(stats.Frees))
	rs.emit("memoryStats.lastGCPauseTimeNS", float64(stats.PauseNs[(stats.NumGC+255)%256]))*/
	m, err := mem.VirtualMemory()
	if err != nil {
		log.Printf("MemStats: failed to emit: %v", err)
	}
	rs.emit("memoryStats.TotalMemory", float64(m.Total))
	rs.emit("memoryStats.AvailableMemory", float64(m.Available))
	rs.emit("memoryStats.UsedMemory", float64(m.Used))
	rs.emit("memoryStats.UsedPercent", float64(m.UsedPercent))
}

func (rs *RuntimeStats) emit(name string, value float64) {
	err := rs.emitter.Emit(&events.ValueMetric{
		Name:  &name,
		Value: &value,
		Unit:  proto.String("count"),
	})
	if err != nil {
		log.Printf("RuntimeStats: failed to emit: %v", err)
	}
}

/*
 Description: VM - CPU Info metrics
 */
func (rs *RuntimeStats) emitCpuMetrics() {
	numcpu := runtime.NumCPU()
	//duration := time.Duration(1) * time.Second
	duration := time.Duration(200) * time.Millisecond
	c, err := cpu.Percent(duration, true)
	if err != nil {
		log.Println("getting cpu metrics error %v", err.Error())
		//log.Fatalf("getting cpu metrics error %v", err)
		return
	}
	for k, percent := range c {
		// Check for slightly greater then 100% to account for any rounding issues.
		if percent < 0.0 || percent > 100.0001 * float64(numcpu) {
			log.Println("CPUPercent value is invalid: %f", percent)
			//log.Fatalf("CPUPercent value is invalid: %f", percent)
		}else{
			rs.emit(fmt.Sprintf("cpuStats.%d", k), float64(percent))
		}
		//log.Println("%d cpu %f", k, percent)
	}
	//============ CPU Load Average : Only support linux & freebsd ==============
	h, err := host.Info()
	if h.OS == "linux" || h.OS == "freebsd"{
		loadAvgStat, err := load.Avg()
		if err != nil {
			log.Printf("LoadAvgStats: failed to emit: %v", err)
		}
		rs.emit("loadavg1.", float64(loadAvgStat.Load1))
		rs.emit("loadavg5.", float64(loadAvgStat.Load5))
		rs.emit("loadavg15.", float64(loadAvgStat.Load15))
	}
	//===========================================================================
}
/*
 Description: VM - Disk/IO Info metrics
 */
func (rs *RuntimeStats) emitDiskMetrics() {
	if runtime.GOOS == "windows" {
		var pathKey []string
		diskios, _ := disk.IOCounters()
		for key, value := range diskios{
			pathKey = append(pathKey, key)
			rs.emit(fmt.Sprintf("diskIOStats.%s.readCount", key), float64(value.ReadCount))
			rs.emit(fmt.Sprintf("diskIOStats.%s.writeCount", key), float64(value.WriteCount))
			rs.emit(fmt.Sprintf("diskIOStats.%s.readBytes", key), float64(value.ReadBytes))
			rs.emit(fmt.Sprintf("diskIOStats.%s.writeBytes", key), float64(value.WriteBytes))
			rs.emit(fmt.Sprintf("diskIOStats.%s.readTime", key), float64(value.ReadTime))
			rs.emit(fmt.Sprintf("diskIOStats.%s.writeTime", key), float64(value.WriteTime))
			rs.emit(fmt.Sprintf("diskIOStats.%s.ioTime", key), float64(value.IoTime))
		}
		//Newly Added - Disk I/O (2017.04)
		for _, value := range pathKey {
			d, err := disk.Usage(value)
			if err != nil {
				log.Println("getting disk info error %v", err.Error())
				//log.Fatalf("getting disk info error %v", err)
				return
			}
			rs.emit(fmt.Sprintf("diskStats.windows.%s.Total", d.Path), float64(d.Total))
			rs.emit(fmt.Sprintf("diskStats.windows.%s.Used", d.Path), float64(d.Used))
			rs.emit(fmt.Sprintf("diskStats.windows.%s.Available", d.Path), float64(d.Free))
			rs.emit(fmt.Sprintf("diskStats.windows.%s.Usage", d.Path), float64(d.UsedPercent))
		}
	}else{
		diskparts, err := disk.Partitions(false)
		if err != nil {
			fmt.Errorf("get disk partitions error: %v", err)
		}
		for _, partition := range diskparts {
			//fmt.Println("partition KEY:", key, "value:", partition)
			if partition.Mountpoint == "/" {
				mountpoints := strings.Split(partition.Device, "/")
				d, err := disk.Usage(partition.Mountpoint)
				if err != nil {
					log.Println("getting disk info error %v", err.Error())
					//log.Fatalf("getting disk info error %v", err)
					return
				}
				//log.Printf("path : %s, fstype : %s, total : %d, used : %d, avail : %d, usage : %f", d.Path, d.Fstype, d.Total, d.Used, d.Free, d.UsedPercent)
				rs.emit("diskStats.Total", float64(d.Total))
				rs.emit("diskStats.Used", float64(d.Used))
				rs.emit("diskStats.Available", float64(d.Free))
				rs.emit("diskStats.Usage", float64(d.UsedPercent))
				//Newly Added - Disk I/O (2017.04)
				diskios, _ := disk.IOCounters()
				for key, value := range diskios {
					if mountpoints[len(mountpoints) - 1] == key {
						//log.Printf("diskio key : %s, diskio value : %v", key, value)
						rs.emit(fmt.Sprintf("diskIOStats.%s.readCount", key), float64(value.ReadCount))
						rs.emit(fmt.Sprintf("diskIOStats.%s.writeCount", key), float64(value.WriteCount))
						rs.emit(fmt.Sprintf("diskIOStats.%s.readBytes", key), float64(value.ReadBytes))
						rs.emit(fmt.Sprintf("diskIOStats.%s.writeBytes", key), float64(value.WriteBytes))
						rs.emit(fmt.Sprintf("diskIOStats.%s.readTime", key), float64(value.ReadTime))
						rs.emit(fmt.Sprintf("diskIOStats.%s.writeTime", key), float64(value.WriteTime))
						rs.emit(fmt.Sprintf("diskIOStats.%s.ioTime", key), float64(value.IoTime))
					}
				}
			}
		}
	}
}
/*
Newly Added - Network I/O (2017.04)
Description: VM - Network Interface & I/O Info metrics
*/
func (rs *RuntimeStats) emitNetworkMetrics() {
	nifs, err := net.Interfaces()
	if err != nil {
		log.Println("getting network interface info error %v", err.Error())
		//log.Fatalf("getting network interface info error %v", err)
		return
	}
	for _, intf := range nifs {
		rs.emit(fmt.Sprintf("networkInterface.%s.%s", intf.Name, "MTU"), float64(intf.MTU))
	}
	ios, err := net.IOCounters(true)
	for _, value := range ios {
		rs.emit(fmt.Sprintf("networkIOStats.%s.bytesRecv", value.Name), float64(value.BytesRecv))
		rs.emit(fmt.Sprintf("networkIOStats.%s.bytesSent", value.Name), float64(value.BytesSent))
		rs.emit(fmt.Sprintf("networkIOStats.%s.packetRecv", value.Name), float64(value.PacketsRecv))
		rs.emit(fmt.Sprintf("networkIOStats.%s.packetSent", value.Name), float64(value.PacketsSent))
		rs.emit(fmt.Sprintf("networkIOStats.%s.dropIn", value.Name), float64(value.Dropin))
		rs.emit(fmt.Sprintf("networkIOStats.%s.dropOut", value.Name), float64(value.Dropout))
		rs.emit(fmt.Sprintf("networkIOStats.%s.errIn", value.Name), float64(value.Errin))
		rs.emit(fmt.Sprintf("networkIOStats.%s.errOut", value.Name), float64(value.Errout))
	}
}
/*
Newly Added - Network I/O (2017.04)
Description: VM - Process Info metrics
*/
func (rs *RuntimeStats) emitProcessMetrics() {
	procs, err := process.Pids()
	if err != nil {
		log.Println("getting processes error %v", err.Error())
		//log.Fatalf("getting processes error %v", err)
		return
	}
	//pStatArray := make([]processStat, 0)
	pStatArray := make(ProcessStatArray, 0)
	for _, value :=range procs{
		p, err := process.NewProcess(value)
		if err != nil {
			log.Println("getting single process info error %v", err.Error())
			//log.Fatalf("getting single process info error %v", err)
			return
		}else{
			var pStat ProcessStat
			ct, _:= p.CreateTime()
			s_timestamp := strconv.FormatInt(ct, 10)
			//window환경에서는 모든 프로세스를 조회하기 때문에 많은 시간이 소요된다.
			//이를 방지하기 위해 프로세스시작 시간을 통해 제어한다.
			if len(s_timestamp) >= 10{
				pStat.startTime = s_timestamp[:10]
				pStat.pid = p.Pid
				pp, _ := p.Ppid()
				pStat.ppid = pp
				pname, _ := p.Name()
				pStat.name = pname
				m,err := p.MemoryInfo()
				if err == nil {
					pStat.memUsage = m.RSS
				}
				pStatArray = append(pStatArray, pStat)
			}
		}
	}
	//Memroy 점유 크기별로 Sorting
	sort.Sort(pStatArray)

	var index int
	for _, ps := range pStatArray {
		//fmt.Println("##### runtime_stats , process info :", ps.name, ps.memUsage, ps.startTime)
		/*if index > 20 {
			break
		}
		rs.emit(fmt.Sprintf("processStats.%d.%s.pid",index, ps.name), float64(ps.pid))
		rs.emit(fmt.Sprintf("processStats.%d.%s.ppid",index, ps.name), float64(ps.ppid))
		if f, err := strconv.ParseFloat(ps.startTime, 64); err == nil {
			rs.emit(fmt.Sprintf("processStats.%d.%s.startTime", index, ps.name),  f)
		}
		rs.emit(fmt.Sprintf("processStats.%d.%s.memUsage",index, ps.name), float64(ps.memUsage))
		index++*/
		if index > 20 {
			break
		}
		if f, err := strconv.ParseInt(ps.startTime, 10, 0); err == nil {
			rs.emit(fmt.Sprintf("processStats.%d.%s.pid.%d.ppid.%d.memUsage.%d.startTime.%d", index, ps.name, ps.pid, ps.ppid, ps.memUsage, f), float64(index))
		}else{
			rs.emit(fmt.Sprintf("processStats.%d.%s.pid.%d.ppid.%d.memUsage.%d.startTime.%d", index, ps.name, ps.pid, ps.ppid, ps.memUsage, 0), float64(index))
		}
		index++
	}
}

// Sorting processStat --------------------------------------START
func (arr ProcessStatArray) Len() int {
	return len(arr)
}

// j 인덱스를 가진녀석이 i 앞으로 와야하는지 말아야하는지를 판단하는 함수
func (arr ProcessStatArray) Less(i, j int) bool {
	if arr[i].memUsage == arr[j].memUsage {
		return arr[i].startTime > arr[j].startTime
	} else {
		return arr[i].memUsage > arr[j].memUsage
	}
}

func (arr ProcessStatArray) Swap(i, j int) {
	arr[i], arr[j] = arr[j], arr[i]
}
//RankingSlice----------------------------------------------END