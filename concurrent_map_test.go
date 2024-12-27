package main

import (
	"fmt"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// testCMap runs a suite of tests against any CMap implementation
func testCMap(t *testing.T, m CMap[string, int]) {
	// Test Set and Get
	t.Run("Set and Get", func(t *testing.T) {
		m.Reset()
		m.Set("one", 1)
		val, exists := m.Get("one")
		assert.True(t, exists, "key should exist")
		assert.Equal(t, 1, val, "value should be 1")
	})

	// Test Del
	t.Run("Delete", func(t *testing.T) {
		m.Reset()
		m.Set("temp", 999)
		m.Del("temp")
		_, exists := m.Get("temp")
		assert.False(t, exists, "key should not exist after deletion")
	})

	// Test Values
	t.Run("Values", func(t *testing.T) {
		m.Reset()
		m.Set("a", 1)
		m.Set("b", 2)
		values := m.Values()
		assert.Len(t, values, 2, "should have 2 values")
		sort.Ints(values)
		assert.EqualValues(t, []int{1, 2}, values, "should contain 1 and 2 in sorted order")

	})

	t.Run("Sequential access", func(t *testing.T) {
		m.Reset()
		var wg sync.WaitGroup

		t.Run("Single Writer Multiple Readers", func(t *testing.T) {
			m.Reset()
			const readers = 5
			wg.Add(readers + 1)

			// Start readers that wait for a specific value
			readyChan := make(chan struct{})
			resultChan := make(chan int, readers)

			// Launch readers that wait for key "test" to equal 42
			for i := 0; i < readers; i++ {
				go func() {
					defer wg.Done()
					<-readyChan // Wait for signal to start reading

					// Read until we get the expected value
					for {
						if val, exists := m.Get("test"); exists && val == 42 {
							resultChan <- 42
							break
						}
						time.Sleep(time.Millisecond)
					}
				}()
			}

			// Writer goroutine
			go func() {
				defer wg.Done()
				m.Set("test", 41)
				close(readyChan) // Signal readers to start
				time.Sleep(10 * time.Millisecond)
				m.Set("test", 42)
			}()

			// Verify all readers got the correct value
			for i := 0; i < readers; i++ {
				val := <-resultChan
				assert.Equal(t, 42, val, "reader should have received 42")
			}
			wg.Wait()
		})

		t.Run("Ordered Writers", func(t *testing.T) {
			m.Reset()
			const writers = 300
			wg.Add(writers)

			// Create channels to coordinate the order of operations
			channels := make([]chan struct{}, writers)
			for i := range channels {
				channels[i] = make(chan struct{})
			}

			// Launch writers in specific order
			for i := 0; i < writers; i++ {
				go func(writerNum int) {
					defer wg.Done()

					// Wait for our turn
					if writerNum > 0 {
						<-channels[writerNum-1]
					}

					// Perform ordered write
					m.Set("sequence", writerNum)

					// Signal next writer
					if writerNum < writers-1 {
						close(channels[writerNum])
					}
				}(i)
			}

			wg.Wait()

			// Verify final value
			val, exists := m.Get("sequence")
			assert.True(t, exists)
			assert.Equal(t, writers-1, val, "final value should be from last writer")
		})

	})

	t.Run("Concurrent access", func(t *testing.T) {
		m.Reset()
		var wg sync.WaitGroup

		t.Run("Verified Updates", func(t *testing.T) {
			m.Reset()
			const updaters = 3
			wg.Add(updaters)

			// Each updater increments its own key
			for i := 0; i < updaters; i++ {
				go func(updaterNum int) {
					defer wg.Done()
					key := fmt.Sprintf("counter_%d", updaterNum)

					// Perform 10 increments
					for j := 0; j < 10; j++ {
						val, _ := m.Get(key)
						m.Set(key, val+1)
						time.Sleep(time.Millisecond) // Small delay to simulate work
					}
				}(i)
			}

			wg.Wait()

			// Verify each counter reached exactly 10
			for i := 0; i < updaters; i++ {
				key := fmt.Sprintf("counter_%d", i)
				val, exists := m.Get(key)
				assert.True(t, exists)
				assert.Equal(t, 10, val, "counter should have been incremented exactly 10 times")
			}
		})

		t.Run("Concurrent Access Safety", func(t *testing.T) {
			m.Reset()
			const (
				numGoroutines = 100
				iterations    = 1000
				keyPrefix     = "key"
			)

			var wg sync.WaitGroup
			wg.Add(numGoroutines)

			for i := 0; i < numGoroutines; i++ {
				go func(id int) {
					defer wg.Done()
					key := fmt.Sprintf("%s_%d", keyPrefix, id)

					// Each goroutine works with its own key
					for j := 0; j < iterations; j++ {
						m.Set(key, j)
						val, exists := m.Get(key)
						assert.True(t, exists)
						assert.Equal(t, j, val)
					}
				}(i)
			}

			wg.Wait()
		})

		t.Run("Readers During Continuous Updates", func(t *testing.T) {
			const (
				numReaders   = 50
				numWriters   = 10
				readDuration = 200 * time.Millisecond
				key          = "value"
			)

			var done sync.WaitGroup
			done.Add(numReaders + numWriters)

			// Track all observed values
			var observedValues sync.Map

			// Launch readers
			for i := 0; i < numReaders; i++ {
				go func(readerID int) {
					defer done.Done()

					seen := make(map[int]bool)
					deadline := time.Now().Add(readDuration)

					// Read continuously until deadline
					for time.Now().Before(deadline) {
						if val, exists := m.Get(key); exists {
							seen[val] = true
						}
					}

					// Record unique values seen by this reader
					observedValues.Store(readerID, seen)
				}(i)
			}

			// Launch writers
			for i := 0; i < numWriters; i++ {
				go func(writerID int) {
					defer done.Done()

					// Write continuously with increasing values
					counter := 0
					deadline := time.Now().Add(readDuration)

					for time.Now().Before(deadline) {
						m.Set(key, counter)
						counter++
					}
				}(i)
			}

			done.Wait()

			// Analyze observed values
			var totalUnique int
			allSeen := make(map[int]int) // value -> number of readers that saw it

			observedValues.Range(func(_, v interface{}) bool {
				seen := v.(map[int]bool)
				for val := range seen {
					allSeen[val]++
					totalUnique = max(totalUnique, val)
				}
				return true
			})

			// Report statistics
			t.Logf("Highest value observed: %d", totalUnique)

			// Count how many values were seen by all readers
			var fullyObserved int
			for _, count := range allSeen {
				if count == numReaders {
					fullyObserved++
				}
			}
			t.Logf("Values seen by all readers: %d", fullyObserved)

			// Basic assertions
			assert.Greater(t, totalUnique, 0, "should have observed some values")
			assert.Greater(t, len(allSeen), 0, "should have recorded observations")
		})

	})
}

func TestConcurrentMap(t *testing.T) {
	t.Run("mutexMap", func(t *testing.T) {
		m := NewMutexMap[string, int]()
		testCMap(t, m)
	})

}
