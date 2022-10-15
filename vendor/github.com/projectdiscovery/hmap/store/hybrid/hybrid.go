package hybrid

import (
	"io/ioutil"
	"os"
	"runtime"
	"time"

	"github.com/projectdiscovery/hmap/store/cache"
	"github.com/projectdiscovery/hmap/store/disk"
)

type MapType int

const (
	Memory = iota
	Disk
	Hybrid
)

type Options struct {
	MemoryExpirationTime time.Duration
	DiskExpirationTime   time.Duration
	JanitorTime          time.Duration
	Type                 MapType
	MemoryGuardForceDisk bool
	MemoryGuard          bool
	MaxMemorySize        int
	MemoryGuardTime      time.Duration
	Path                 string
	Cleanup              bool
}

var DefaultOptions = Options{
	Type:                 Memory,
	MemoryExpirationTime: time.Duration(5) * time.Minute,
	JanitorTime:          time.Duration(1) * time.Minute,
}

var DefaultMemoryOptions = Options{
	Type: Memory,
}

var DefaultDiskOptions = Options{
	Type:    Disk,
	Cleanup: true,
}

var DefaultHybridOptions = Options{
	Type:                 Hybrid,
	MemoryExpirationTime: time.Duration(5) * time.Minute,
	JanitorTime:          time.Duration(1) * time.Minute,
}

type HybridMap struct {
	options     *Options
	memorymap   cache.Cache
	diskmap     disk.DB
	diskmapPath string
	memoryguard *memoryguard
}

func New(options Options) (*HybridMap, error) {
	var hm HybridMap
	if options.Type == Memory || options.Type == Hybrid {
		hm.memorymap = cache.New(options.MemoryExpirationTime, options.JanitorTime)
	}

	if options.Type == Disk || options.Type == Hybrid {
		diskmapPathm := options.Path
		if diskmapPathm == "" {
			var err error
			diskmapPathm, err = ioutil.TempDir("", "hm")
			if err != nil {
				return nil, err
			}
		}

		hm.diskmapPath = diskmapPathm
		db, err := disk.OpenLevelDB(diskmapPathm)
		if err != nil {
			return nil, err
		}
		hm.diskmap = db
	}

	if options.Type == Hybrid {
		hm.memorymap.OnEvicted(func(k string, v interface{}) {
			hm.diskmap.Set(k, v.([]byte), 0)
		})
	}

	if options.MemoryGuard {
		runMemoryGuard(&hm, options.MemoryGuardTime)
		runtime.SetFinalizer(&hm, stopMemoryGuard)
	}

	hm.options = &options

	return &hm, nil
}

func (hm *HybridMap) Close() error {
	hm.diskmap.Close()
	if hm.diskmapPath != "" && hm.options.Cleanup {
		return os.RemoveAll(hm.diskmapPath)
	}
	return nil
}

func (hm *HybridMap) Set(k string, v []byte) error {
	switch hm.options.Type {
	case Hybrid:
		fallthrough
	case Memory:
		if hm.options.MemoryGuardForceDisk {
			hm.diskmap.Set(k, v, hm.options.DiskExpirationTime)
		} else {
			hm.memorymap.Set(k, v)
		}
	case Disk:
		hm.diskmap.Set(k, v, hm.options.DiskExpirationTime)
	}

	return nil
}

func (hm *HybridMap) Get(k string) ([]byte, bool) {
	switch hm.options.Type {
	case Memory:
		v, ok := hm.memorymap.Get(k)
		if ok {
			return v.([]byte), ok
		}
		return []byte{}, ok
	case Hybrid:
		v, ok := hm.memorymap.Get(k)
		if ok {
			return v.([]byte), ok
		}
		vm, err := hm.diskmap.Get(k)
		// load it in memory since it has been recently used
		if err == nil {
			hm.memorymap.Set(k, vm)
		}
		return vm, err == nil
	case Disk:
		v, err := hm.diskmap.Get(k)
		return v, err == nil
	}

	return []byte{}, false
}

func (hm *HybridMap) Del(key string) error {
	switch hm.options.Type {
	case Memory:
		hm.memorymap.Delete(key)
	case Hybrid:
		hm.memorymap.Delete(key)
		return hm.diskmap.Del(key)
	case Disk:
		return hm.diskmap.Del(key)
	}

	return nil
}

func (hm *HybridMap) Scan(f func([]byte, []byte) error) {
	switch hm.options.Type {
	case Memory:
		hm.memorymap.Scan(f)
	case Hybrid:
		hm.memorymap.Scan(f)
		hm.diskmap.Scan(disk.ScannerOptions{Handler: f})
	case Disk:
		hm.diskmap.Scan(disk.ScannerOptions{Handler: f})
	}
}

func (hm *HybridMap) Size() int64 {
	return 0
}

func (hm *HybridMap) TuneMemory() {
	// si := sysinfo.Get()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	if m.Alloc >= uint64(hm.options.MaxMemorySize) {
		hm.options.MemoryGuardForceDisk = true
	} else {
		hm.options.MemoryGuardForceDisk = false
	}
}
