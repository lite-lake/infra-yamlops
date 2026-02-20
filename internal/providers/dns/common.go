package dns

import "fmt"

func EnsureRecord(provider Provider, domain string, desired *DNSRecord) error {
	records, err := provider.ListRecords(domain)
	if err != nil {
		return fmt.Errorf("list records: %w", err)
	}

	for _, existing := range records {
		if existing.Type == desired.Type && existing.Name == desired.Name {
			if existing.Value == desired.Value && existing.TTL == desired.TTL {
				return nil
			}
			return provider.UpdateRecord(domain, existing.ID, desired)
		}
	}
	return provider.CreateRecord(domain, desired)
}
