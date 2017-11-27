package routingtable

type MessagesToEmit struct {
	RegistrationMessages   []RegistryMessage
	UnregistrationMessages []RegistryMessage
}

func (m MessagesToEmit) Merge(o MessagesToEmit) MessagesToEmit {
	return MessagesToEmit{
		RegistrationMessages:   append(m.RegistrationMessages, o.RegistrationMessages...),
		UnregistrationMessages: append(m.UnregistrationMessages, o.UnregistrationMessages...),
	}
}

func (m MessagesToEmit) RouteRegistrationCount() uint64 {
	return routeCount(m.RegistrationMessages)
}

func (m MessagesToEmit) RouteUnregistrationCount() uint64 {
	return routeCount(m.UnregistrationMessages)
}

func routeCount(messages []RegistryMessage) uint64 {
	var count uint64
	for _, message := range messages {
		count += uint64(len(message.URIs))
	}
	return count
}
