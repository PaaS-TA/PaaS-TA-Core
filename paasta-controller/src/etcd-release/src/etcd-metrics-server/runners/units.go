package runners

const (
	MetricUnit            = "Metric"
	BytesPerSecondUnit    = "B/s"
	RequestsPerSecondUnit = "Req/s"
)

var (
	unitMap = map[string]string{
		"SendingRequestRate":     RequestsPerSecondUnit,
		"SendingBandwidthRate":   BytesPerSecondUnit,
		"ReceivingRequestRate":   RequestsPerSecondUnit,
		"ReceivingBandwidthRate": BytesPerSecondUnit,
	}
)

func GetMetricUnit(metric string) string {
	unit, ok := unitMap[metric]

	if !ok {
		return MetricUnit
	} else {
		return unit
	}
}
