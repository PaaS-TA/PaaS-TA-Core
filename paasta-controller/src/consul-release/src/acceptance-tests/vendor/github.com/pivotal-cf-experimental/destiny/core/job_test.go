package core_test

import (
	"github.com/pivotal-cf-experimental/destiny/core"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Job", func() {
	Describe("HasTemplate", func() {
		It("returns true if the job has the template", func() {
			job := core.Job{
				Templates: []core.JobTemplate{
					{
						Name:    "some-template-name",
						Release: "some-template-release",
					},
				},
			}

			Expect(job.HasTemplate("some-template-name", "some-template-release")).To(BeTrue())
		})

		It("returns false if the job does not have the template", func() {
			job := core.Job{
				Templates: []core.JobTemplate{
					{
						Name:    "some-other-template-name",
						Release: "some-other-template-release",
					},
				},
			}

			Expect(job.HasTemplate("some-template-name", "some-template-release")).To(BeFalse())
		})
	})
})
