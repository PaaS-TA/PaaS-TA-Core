/*
 * Cloud Foundry 2012.02.03 Beta
 * Copyright (c) [2009-2012] VMware, Inc. All Rights Reserved.
 *
 * This product is licensed to you under the Apache License, Version 2.0 (the "License").
 * You may not use this product except in compliance with the License.
 *
 * This product includes a number of subcomponents with
 * separate copyright notices and license terms. Your use of these
 * subcomponents is subject to the terms and conditions of the
 * subcomponent's license, as noted in the LICENSE file.
 */

package org.cloudfoundry.identity.uaa.scim.job;

import java.util.Collections;
import java.util.List;
import java.util.Map;

import org.apache.commons.logging.Log;
import org.apache.commons.logging.LogFactory;
import org.springframework.batch.classify.SubclassClassifier;
import org.springframework.batch.core.ExitStatus;
import org.springframework.batch.core.StepExecution;
import org.springframework.batch.core.StepExecutionListener;
import org.springframework.batch.core.listener.SkipListenerSupport;
import org.springframework.batch.item.ItemWriter;

/**
 * @author Dave Syer
 * 
 */
public class ItemWriterSkipListener<T, S> extends SkipListenerSupport<T, S> implements StepExecutionListener {

	private static Log logger = LogFactory.getLog(ItemWriterSkipListener.class);

	private static class CounterWriter<O> implements ItemWriter<O> {

		private int count = 0;

		@Override
		public void write(List<? extends O> items) throws Exception {
			if (count < 100) {
				logger.debug("Skipped item: " + items.get(0));
			}
			count++;
		}

	}

	private CounterWriter<S> counter = new CounterWriter<S>();

	private SubclassClassifier<Throwable, ItemWriter<S>> classifier = new SubclassClassifier<Throwable, ItemWriter<S>>(
			counter);

	public void setWriters(Map<Class<? extends Throwable>, ItemWriter<S>> map) {
		classifier.setTypeMap(map);
	}

	@Override
	public void onSkipInWrite(S item, Throwable t) {
		logger.debug("Skipping: " + item + "(" + t.getClass().getName() + ", " + t.getMessage() + ")");
		List<S> items = Collections.singletonList(item);
		try {
			classifier.classify(t).write(items);
		}
		catch (Exception e) {
			try {
				classifier.getDefault().write(items);
			}
			catch (Exception ex) {
				// ignore
				logger.error("Could not register failed item", ex);
			}
		}
	}

	@Override
	public void beforeStep(StepExecution stepExecution) {
	}

	@Override
	public ExitStatus afterStep(StepExecution stepExecution) {
		if (counter.count > 0) {
			logger.warn("Skipped " + counter.count + " records with no recovery");
		}
		return stepExecution.getExitStatus();
	}

}
