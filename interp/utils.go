package interp

import (
	"errors"

	"tinygo.org/x/go-llvm"
)

// Return a list of values (actually, instructions) where this value is used as
// an operand.
func getUses(value llvm.Value) []llvm.Value {
	var uses []llvm.Value
	use := value.FirstUse()
	for !use.IsNil() {
		uses = append(uses, use.User())
		use = use.NextUse()
	}
	return uses
}

// getStringBytes loads the byte slice of a Go string represented as a
// {ptr, len} pair.
func getStringBytes(strPtr Value, strLen llvm.Value) ([]byte, error) {
	if !strLen.IsConstant() {
		return nil, errors.New("getStringBytes with a non-constant length")
	}
	buf := make([]byte, strLen.ZExtValue())
	for i := range buf {
		gep, err := strPtr.GetElementPtr([]uint32{uint32(i)})
		if err != nil {
			return nil, err
		}
		c, err := gep.Load()
		if err != nil {
			return nil, err
		}
		buf[i] = byte(c.ZExtValue())
	}
	return buf, nil
}

// getLLVMIndices converts an []uint32 into an []llvm.Value, for use in
// llvm.ConstGEP.
func getLLVMIndices(int32Type llvm.Type, indices []uint32) []llvm.Value {
	llvmIndices := make([]llvm.Value, len(indices))
	for i, index := range indices {
		llvmIndices[i] = llvm.ConstInt(int32Type, uint64(index), false)
	}
	return llvmIndices
}

// Return true if this type is a scalar value (integer or floating point), false
// otherwise.
func isScalar(t llvm.Type) bool {
	switch t.TypeKind() {
	case llvm.IntegerTypeKind, llvm.FloatTypeKind, llvm.DoubleTypeKind:
		return true
	default:
		return false
	}
}

// isPointerNil returns whether this is a nil pointer or not. The ok value
// indicates whether the result is certain: if it is false the result boolean is
// not valid.
func isPointerNil(v llvm.Value) (result bool, ok bool) {
	if !v.IsAConstantExpr().IsNil() {
		switch v.Opcode() {
		case llvm.IntToPtr:
			// Whether a constant inttoptr is nil is easy to
			// determine.
			result, ok = isZero(v.Operand(0))
			if ok {
				return
			}
		case llvm.BitCast, llvm.GetElementPtr:
			// These const instructions are just a kind of wrappers for the
			// underlying pointer.
			return isPointerNil(v.Operand(0))
		}
	}
	if !v.IsAConstantPointerNull().IsNil() {
		// A constant pointer null is always null, of course.
		return true, true
	}
	if !v.IsAGlobalValue().IsNil() {
		// A global value is never null.
		return false, true
	}
	return false, false // not valid
}

// isZero returns whether the value in v is the integer zero, and whether that
// can be known right now.
func isZero(v llvm.Value) (result bool, ok bool) {
	if !v.IsAConstantExpr().IsNil() {
		switch v.Opcode() {
		case llvm.PtrToInt:
			return isPointerNil(v.Operand(0))
		}
	}
	if !v.IsAConstantInt().IsNil() {
		val := v.ZExtValue()
		return val == 0, true
	}
	return false, false // not valid
}

// unwrap returns the underlying value, with GEPs removed. This can be useful to
// get the underlying global of a GEP pointer.
func unwrap(value llvm.Value) llvm.Value {
	for {
		if !value.IsAConstantExpr().IsNil() {
			switch value.Opcode() {
			case llvm.GetElementPtr:
				value = value.Operand(0)
				continue
			}
		} else if !value.IsAGetElementPtrInst().IsNil() {
			value = value.Operand(0)
			continue
		}
		break
	}
	return value
}
