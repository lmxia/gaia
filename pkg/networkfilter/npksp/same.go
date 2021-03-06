package npksp

import "unsafe"

// same determines whether two sets are backed by the same store. In the
// current implementation using hash maps it makes use of the fact that
// hash maps are passed as a pointer to a runtime Hmap struct. A map is
// not seen by the runtime as a pointer though, so we use unsafe to get
// the maps' pointer values to compare.
func same(a, b Nodes) bool {
	return *(*uintptr)(unsafe.Pointer(&a)) == *(*uintptr)(unsafe.Pointer(&b))
}

// intsSame determines whether two sets are backed by the same store. In the
// current implementation using hash maps it makes use of the fact that
// hash maps are passed as a pointer to a runtime Hmap struct. A map is
// not seen by the runtime as a pointer though, so we use unsafe to get
// the maps' pointer values to compare.
func intsSame(a, b Ints) bool {
	return *(*uintptr)(unsafe.Pointer(&a)) == *(*uintptr)(unsafe.Pointer(&b))
}

// int64sSame determines whether two sets are backed by the same store. In the
// current implementation using hash maps it makes use of the fact that
// hash maps are passed as a pointer to a runtime Hmap struct. A map is
// not seen by the runtime as a pointer though, so we use unsafe to get
// the maps' pointer values to compare.
func int64sSame(a, b Int64s) bool {
	return *(*uintptr)(unsafe.Pointer(&a)) == *(*uintptr)(unsafe.Pointer(&b))
}
