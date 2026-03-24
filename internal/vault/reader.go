package vault

// Reader provides read access to vault secrets.
// Consumers should depend on this interface rather than the concrete KDBXVault type.
type Reader interface {
	Get(name string) (string, error)
	GetField(entryName, fieldKey string) (string, error)
	GetAttachment(entryName, attachmentName string) ([]byte, error)
}
