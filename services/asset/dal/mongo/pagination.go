package mongo

func normalizePage(pageNum, pageSize int32) (int64, int64) {
	if pageNum < 1 {
		pageNum = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	return int64((pageNum - 1) * pageSize), int64(pageSize)
}
