package promql_test

import (
	"math"
	"strconv"
	"time"

	"code.cloudfoundry.org/log-cache/internal/promql"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("PromQL Parsing", func() {
	Describe("ParseStep", func() {
		DescribeTable("it supports all Prometheus-accepted units",
			func(stringStep string, expectedStep time.Duration) {
				step, err := promql.ParseStep(stringStep)

				Expect(err).ToNot(HaveOccurred())
				Expect(step).To(Equal(expectedStep))
			},

			Entry("unlabeled second", "1", time.Second),
			Entry("float second", "1.5", 1500*time.Millisecond),
			Entry("second", "1s", time.Second),
			Entry("minute", "1m", time.Minute),
			Entry("hour", "1h", time.Hour),
			Entry("day", "1d", typicalNumberOfHoursPerDay*time.Hour),
			Entry("week", "1w", typicalNumberOfHoursPerWeek*time.Hour),
			Entry("year", "1y", typicalNumberOfHoursPerYear*time.Hour),
		)

		DescribeTable("it handles errors",
			func(stringStep string) {
				_, err := promql.ParseStep(stringStep)

				Expect(err).To(HaveOccurred())
			},

			Entry("empty step", ""),
			Entry("invalid unit", "4q"),
			Entry("overflows int64", strconv.Itoa(math.MaxInt64)+"0"),
		)
	})

	Describe("ParseDuration", func() {
		DescribeTable("it supports all Prometheus-accepted units",
			func(stringDuration string, expectedDuration time.Duration) {
				step, err := promql.ParseDuration(stringDuration)

				Expect(err).ToNot(HaveOccurred())
				Expect(step).To(Equal(expectedDuration))
			},

			Entry("second", "1s", time.Second),
			Entry("minute", "1m", time.Minute),
			Entry("hour", "1h", time.Hour),
			Entry("day", "1d", typicalNumberOfHoursPerDay*time.Hour),
			Entry("week", "1w", typicalNumberOfHoursPerWeek*time.Hour),
			Entry("year", "1y", typicalNumberOfHoursPerYear*time.Hour),
		)

		DescribeTable("it handles errors",
			func(stringDuration string) {
				_, err := promql.ParseDuration(stringDuration)

				Expect(err).To(HaveOccurred())
			},

			Entry("unlabeled second", "1"),
			Entry("float second", "1.5"),
			Entry("empty step", ""),
			Entry("invalid unit", "4q"),
			Entry("overflows int64", strconv.Itoa(math.MaxInt64)+"0"),
		)
	})

	Describe("ParseTime", func() {
		DescribeTable("it supports all Prometheus-accepted formats",
			func(inputTime string, expectedResult time.Time) {
				result, err := promql.ParseTime(inputTime)

				Expect(err).ToNot(HaveOccurred())
				Expect(result).To(BeTemporally("==", expectedResult))
			},

			Entry("whole seconds", "123456", time.Unix(123456, 0)),
			Entry("decimal seconds", "123456.789", time.Unix(123456, 789000000)),
			Entry("RFC3339 with UTC", "2015-07-01T20:10:30.781Z", time.Date(2015, 7, 1, 20, 10, 30, 781000000, time.UTC)),
			Entry("RFC3339 with MST", "2015-07-01T20:10:30.781-06:00", time.Date(2015, 7, 1, 20, 10, 30, 781000000, time.FixedZone("MST", -6*60*60))),
		)

		DescribeTable("it handles errors",
			func(inputTime string) {
				_, err := promql.ParseTime(inputTime)

				Expect(err).To(HaveOccurred())
			},

			Entry("empty time", ""),
			Entry("not a number", "potato"),
			Entry("RFC3339 without timezone", "2015-07-01T20:10:30.781"),
		)
	})
})

const (
	typicalNumberOfHoursPerDay  = 24
	typicalNumberOfHoursPerWeek = typicalNumberOfHoursPerDay * 7
	typicalNumberOfHoursPerYear = typicalNumberOfHoursPerDay * 365
)
