package waste

import (
	"runtime"
	"sync"
	"time"

	"github.com/by275/noidle/controller"
	"github.com/by275/noidle/internal/log"
	"github.com/shirou/gopsutil/v4/cpu"
	"go.einride.tech/pid"
	"golang.org/x/crypto/chacha20"
)

func CPUPercent(referencePercent float64) {
	maxStep := 100000.0
	rateImpact := maxStep / 1000
	machine := newMachine(maxStep)
	machine.controller = controller.RunPID(machine, referencePercent, rateImpact, false)
}

type machine struct {
	runtimePeriod   time.Duration // ms
	maxControlValue float64
	busyTime        int64
	idleTime        time.Duration

	revolution   float64
	lastMeasured float64
	mu           sync.RWMutex
	controller   *pid.Controller
}

func newMachine(maxStep float64) *machine {
	e := &machine{runtimePeriod: time.Second, maxControlValue: maxStep}
	e.busyTime = 0
	e.idleTime = time.Duration(e.maxControlValue)
	for i := 0; i < runtime.NumCPU(); i++ {
		go e.Run()
	}
	return e
}

func (m *machine) Run() {
	buffer, cipher := newCPUBufferAndCipher()
	for {
		busyTime, idleTime := m.currentTimings()
		startTime := time.Now().UnixNano()
		for time.Now().UnixNano()-startTime < busyTime {
			cipher.XORKeyStream(buffer, buffer)
			newCipher, err := chacha20.NewUnauthenticatedCipher(buffer[:32], buffer[:24])
			if err == nil {
				cipher = newCipher
			}
		}
		time.Sleep(idleTime)
	}
}

func (m *machine) Measure() float64 {
	percent, err := cpu.Percent(time.Second, false)
	if err != nil {
		log.Logf("PID", "failed to measure CPU usage, reusing last value: %v", err)
		return m.lastMeasuredValue()
	}

	m.setLastMeasuredValue(percent[0])
	return percent[0]
}

func (m *machine) Control(value float64) {
	// value range [0, maxControlValue] (unit: Nanosecond)
	m.mu.Lock()
	defer m.mu.Unlock()

	m.revolution += value
	if m.revolution < 0 {
		m.revolution = 0
		if m.controller != nil {
			m.controller.Reset()
		}
	} else if m.revolution > m.maxControlValue {
		m.revolution = m.maxControlValue
	}
	value = m.revolution / m.maxControlValue
	totalTime := m.runtimePeriod.Nanoseconds()
	m.busyTime = int64(float64(totalTime) * value)
	m.idleTime = time.Duration(totalTime - m.busyTime)
}

func (m *machine) currentTimings() (int64, time.Duration) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.busyTime, m.idleTime
}

func (m *machine) lastMeasuredValue() float64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.lastMeasured
}

func (m *machine) setLastMeasuredValue(value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.lastMeasured = value
}
