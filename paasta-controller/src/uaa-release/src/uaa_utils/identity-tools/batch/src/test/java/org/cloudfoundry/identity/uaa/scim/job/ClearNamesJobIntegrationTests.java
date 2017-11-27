/**
 * Cloud Foundry 2012.02.03 Beta Copyright (c) [2009-2012] VMware, Inc. All Rights Reserved.
 * 
 * This product is licensed to you under the Apache License, Version 2.0 (the "License"). You may not use this product
 * except in compliance with the License.
 * 
 * This product includes a number of subcomponents with separate copyright notices and license terms. Your use of these
 * subcomponents is subject to the terms and conditions of the subcomponent's license, as noted in the LICENSE file.
 */

package org.cloudfoundry.identity.uaa.scim.job;

import static org.junit.Assert.assertEquals;

import java.util.Date;
import java.util.Iterator;

import org.cloudfoundry.identity.uaa.test.TestUtils;
import org.junit.Test;
import org.springframework.batch.core.BatchStatus;
import org.springframework.batch.core.Job;
import org.springframework.batch.core.JobExecution;
import org.springframework.batch.core.JobParametersBuilder;
import org.springframework.batch.core.StepExecution;
import org.springframework.beans.factory.annotation.Autowired;
import org.springframework.beans.factory.annotation.Qualifier;
import org.springframework.jdbc.core.JdbcTemplate;

/**
 * @author Dave Syer
 * 
 */
public class ClearNamesJobIntegrationTests extends AbstractJobIntegrationTests {

	@Autowired
	@Qualifier("clearNamesJob")
	private Job job;

	@Test
	public void testJobRuns() throws Exception {
		TestUtils.deleteFrom(cloudControllerDataSource, "users");
		TestUtils.deleteFrom(uaaDataSource, "users");
		JdbcTemplate uaaTemplate = new JdbcTemplate(uaaDataSource);
		uaaTemplate.update("insert into users "
				+ "(id, active, userName, email, password, familyName, givenName, created, lastModified) "
				+ "values (?, ?, ?, ?, ?, ?, ?, ?, ?)", "FOO", true, "uniqua", "uniqua@test.org", "ENCRYPT_ME", "Una",
				"Uniqua", new Date(), new Date());
		uaaTemplate.update("insert into users "
				+ "(id, active, userName, email, password, familyName, givenName, created, lastModified) "
				+ "values (?, ?, ?, ?, ?, ?, ?, ?, ?)", "BAR", true, "username", "uniqua@test.org", "ENCRYPT_ME", "uniqua",
				"test.org", new Date(), new Date());
		uaaTemplate.update("insert into users "
				+ "(id, active, userName, email, password, familyName, givenName, created, lastModified) "
				+ "values (?, ?, ?, ?, ?, ?, ?, ?, ?)", "SPAM", true, "another", "uniqua@test.org", "ENCRYPT_ME", "another",
				"another", new Date(), new Date());
		JobExecution execution = jobLauncher.run(job, new JobParametersBuilder().addDate("start.date", new Date(0L))
				.toJobParameters());
		assertEquals(BatchStatus.COMPLETED, execution.getStatus());
		Iterator<StepExecution> iterator = execution.getStepExecutions().iterator();
		StepExecution step = iterator.next();
		assertEquals(3, step.getReadCount());
		assertEquals(2, step.getWriteCount());
		assertEquals(2, uaaTemplate.queryForInt("select count(*) from users where givenName is null"));
	}
}
