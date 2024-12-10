package entities

import (
	"errors"
	"github.com/KyberNetwork/int256"
)

const (
	ZeroValueTickIndex       = 0
	ZeroValueTickInitialized = false
)

var (
	ErrZeroTickSpacing    = errors.New("tick spacing must be greater than 0")
	ErrInvalidTickSpacing = errors.New("invalid tick spacing")
	ErrZeroNet            = errors.New("tick net delta must be zero")
	ErrSorted             = errors.New("ticks must be sorted")
	ErrEmptyTickList      = errors.New("empty tick list")
	ErrBelowSmallest      = errors.New("below smallest")
	ErrAtOrAboveLargest   = errors.New("at or above largest")
	ErrInvalidTickIndex   = errors.New("invalid tick index")
)

var (
	EmptyTick = Tick{}
)

func ValidateList(ticks []Tick, tickSpacing int) error {
	if tickSpacing <= 0 {
		return ErrZeroTickSpacing
	}

	// ensure ticks are spaced appropriately
	for _, t := range ticks {
		if t.Index%tickSpacing != 0 {
			return ErrInvalidTickSpacing
		}
	}

	// ensure tick liquidity deltas sum to 0
	sum := int256.NewInt(0)
	for _, tick := range ticks {
		sum.Add(sum, tick.LiquidityNet)
	}
	if !sum.IsZero() {
		return ErrZeroNet
	}

	if !isTicksSorted(ticks) {
		return ErrSorted
	}

	return nil
}

func IsBelowSmallest(ticks []Tick, tick int) (bool, error) {
	if len(ticks) == 0 {
		return true, ErrEmptyTickList
	}

	return tick < ticks[0].Index, nil
}

func IsAtOrAboveLargest(ticks []Tick, tick int) (bool, error) {
	if len(ticks) == 0 {
		return true, ErrEmptyTickList
	}

	return tick >= ticks[len(ticks)-1].Index, nil
}

func GetTick(ticks []Tick, index int) (Tick, error) {
	tickIndex, err := binarySearch(ticks, index)
	if err != nil {
		return EmptyTick, err
	}

	if tickIndex < 0 {
		return EmptyTick, ErrInvalidTickIndex
	}

	tick := ticks[tickIndex]

	return tick, nil
}

func NextInitializedTick(ticks []Tick, tick int, lte bool) (Tick, error) {
	if lte {
		isBelowSmallest, err := IsBelowSmallest(ticks, tick)
		if err != nil {
			return EmptyTick, err
		}

		if isBelowSmallest {
			return EmptyTick, ErrBelowSmallest
		}

		isAtOrAboveLargest, err := IsAtOrAboveLargest(ticks, tick)
		if err != nil {
			return EmptyTick, err
		}

		if isAtOrAboveLargest {
			return ticks[len(ticks)-1], nil
		}

		index, err := binarySearch(ticks, tick)
		if err != nil {
			return EmptyTick, err
		}

		return ticks[index], nil
	} else {
		isAtOrAboveLargest, err := IsAtOrAboveLargest(ticks, tick)
		if err != nil {
			return EmptyTick, err
		}

		if isAtOrAboveLargest {
			return EmptyTick, ErrAtOrAboveLargest
		}

		isBelowSmallest, err := IsBelowSmallest(ticks, tick)

		if err != nil {
			return EmptyTick, err
		}

		if isBelowSmallest {
			return ticks[0], nil
		}

		index, err := binarySearch(ticks, tick)
		if err != nil {
			return EmptyTick, err
		}

		return ticks[index+1], nil
	}
}

func NextInitializedTickWithinOneWord(ticks []Tick, tick int, lte bool, tickSpacing int) (int, bool, error) {
	compressed := tick / tickSpacing
	if (tick < 0 && tick % tickSpacing != 0) {
		compressed--; // round towards negative infinity
	}

    position := func(tick int) int {
        return int(uint8(tick) % 0xff)
    }

	if lte {
		bitPos := position(compressed)

		minimum := (compressed - bitPos) * tickSpacing
		// minimum := (wordPos << 8) * tickSpacing

		isBelowSmallest, err := IsBelowSmallest(ticks, tick)
		if err != nil {
			return ZeroValueTickIndex, ZeroValueTickInitialized, err
		}

		if isBelowSmallest {
			return minimum, ZeroValueTickInitialized, ErrBelowSmallest
		}

		nextInitializedTick, err := NextInitializedTick(ticks, tick, lte)
		if err != nil {
			return ZeroValueTickIndex, ZeroValueTickInitialized, err
		}

		index := nextInitializedTick.Index
		nextInitializedTickIndex := max(minimum, index)
		return nextInitializedTickIndex, nextInitializedTickIndex == index, nil
	} else {
		bitPos := position(compressed+1)

		// maximum := ((wordPos+1)<<8)*tickSpacing - 1 //old way result not like uni3
		maximum := (compressed+1 + (255 - bitPos))*tickSpacing //to calc like uni3

		isAtOrAboveLargest, err := IsAtOrAboveLargest(ticks, tick)
		if err != nil {
			return ZeroValueTickIndex, ZeroValueTickInitialized, err
		}

		if isAtOrAboveLargest {
			return maximum, ZeroValueTickInitialized, ErrAtOrAboveLargest
		}

		nextInitializedTick, err := NextInitializedTick(ticks, tick, lte)
		if err != nil {
			return ZeroValueTickIndex, ZeroValueTickInitialized, err
		}

		index := nextInitializedTick.Index
		nextInitializedTickIndex := min(maximum, index)
		return nextInitializedTickIndex, nextInitializedTickIndex == index, nil
	}
}

func NextInitializedTickIndex(ticks []Tick, tick int, lte bool) (int, bool, error) {
	nextInitializedTick, err := NextInitializedTick(ticks, tick, lte)
	if err != nil {
		return ZeroValueTickIndex, ZeroValueTickInitialized, err
	}

	var isInitialized bool
	if !nextInitializedTick.LiquidityGross.IsZero() {
		isInitialized = true
	}

	return nextInitializedTick.Index, isInitialized, nil
}

// utils

func isTicksSorted(ticks []Tick) bool {
	for i := 0; i < len(ticks)-1; i++ {
		if ticks[i].Index > ticks[i+1].Index {
			return false
		}
	}
	return true
}

/**
 * Finds the largest tick in the list of ticks that is less than or equal to tick
 * @param ticks list of ticks
 * @param tick tick to find the largest tick that is less than or equal to tick
 * @private
 */
func binarySearch(ticks []Tick, tick int) (int, error) {
	isBelowSmallest, err := IsBelowSmallest(ticks, tick)
	if err != nil {
		return ZeroValueTickIndex, err
	}

	if isBelowSmallest {
		return ZeroValueTickIndex, ErrBelowSmallest
	}

	// binary search
	start := 0
	end := len(ticks) - 1
	for start <= end {
		mid := (start + end) / 2
		if ticks[mid].Index == tick {
			return mid, nil
		} else if ticks[mid].Index < tick {
			start = mid + 1
		} else {
			end = mid - 1
		}
	}

	// if we get here, we didn't find a tick that is less than or equal to tick
	// so we return the index of the tick that is closest to tick
	if ticks[start].Index < tick {
		return start, nil
	} else {
		return start - 1, nil
	}
}
