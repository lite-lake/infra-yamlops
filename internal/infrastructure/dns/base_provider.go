package dns

import (
	"context"

	"github.com/lite-lake/infra-yamlops/internal/constants"
)

type BaseProvider struct {
	defaultTTL int
}

func NewBaseProvider() *BaseProvider {
	return &BaseProvider{
		defaultTTL: constants.DefaultDNSRecordTTL,
	}
}

func (b *BaseProvider) NormalizeTTL(ttl int) int {
	if ttl == 0 {
		ttl = b.defaultTTL
	}
	return NormalizeTTL(ttl)
}

func (b *BaseProvider) SetDefaultTTL(ttl int) {
	b.defaultTTL = ttl
}

type PaginatedListFunc[T any] func(pageNumber int64, pageSize int64) (records []T, totalCount int64, err error)

type RecordMapper[T any] func(apiRecord T) DNSRecord

func (b *BaseProvider) PaginateAndCollect(ctx context.Context, initialPage int64, pageSize int64, listFn PaginatedListFunc[any], mapper RecordMapper[any]) ([]DNSRecord, error) {
	var records []DNSRecord
	pageNumber := initialPage

	for {
		apiRecords, totalCount, err := listFn(pageNumber, pageSize)
		if err != nil {
			return nil, err
		}

		for _, r := range apiRecords {
			records = append(records, mapper(r))
		}

		if int64(len(records)) >= totalCount {
			break
		}
		pageNumber++
	}
	return records, nil
}

type OffsetPaginatedListFunc[T any] func(offset uint64, limit uint64) (records []T, totalCount uint64, err error)

func (b *BaseProvider) PaginateWithOffset(ctx context.Context, initialOffset uint64, limit uint64, listFn OffsetPaginatedListFunc[any], mapper RecordMapper[any]) ([]DNSRecord, error) {
	var records []DNSRecord
	offset := initialOffset

	for {
		apiRecords, totalCount, err := listFn(offset, limit)
		if err != nil {
			return nil, err
		}

		for _, r := range apiRecords {
			records = append(records, mapper(r))
		}

		if uint64(len(records)) >= totalCount {
			break
		}
		offset += limit
	}
	return records, nil
}
