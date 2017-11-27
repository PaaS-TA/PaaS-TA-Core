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

import java.util.HashMap;
import java.util.Map;

import org.apache.commons.logging.Log;
import org.apache.commons.logging.LogFactory;
import org.springframework.batch.item.ItemProcessor;

/**
 * Item processor that filters out Uaa user records that have correct looking names (not guessed from the email or
 * username).
 * 
 * @author Dave Syer
 * 
 */
public class KosherNameFilterProcessor implements ItemProcessor<Map<String, ?>, Map<String, ?>> {

	private static Log logger = LogFactory.getLog(KosherNameFilterProcessor.class);

	@Override
	public Map<String, ?> process(Map<String, ?> item) throws Exception {

		Map<String, Object> map = new HashMap<String, Object>();
		
		map.put("id", item.get("ID"));

		String userName = (String) item.get("USERNAME");
		map.put("userName", userName);
		String email = getEmail(userName, (String) item.get("EMAIL"));
		map.put("email", email);

		String givenName = (String) item.get("GIVENNAME");
		String familyName = (String) item.get("FAMILYNAME");

		if (!nameIsGenerated(givenName, familyName, email, userName)) {
			return null;
		}

		return map;

	}

	private boolean nameIsGenerated(String givenName, String familyName, String email, String userName) {
		String[] names = getNames(email);
		if (names[0].equals(givenName) && names[1].equals(familyName)) {
			return true;
		}
		if (names[1].equals(givenName) && names[0].equals(familyName)) {
			return true;
		}
		if (userName.equals(givenName) && userName.equals(familyName)) {
			return true;
		}
		if (email.equals(givenName) || email.equals(familyName)) {
			return true;
		}
		return false;
	}

	private String getEmail(String id, String email) {
		if (email == null || !email.contains("@")) {
			String msg = "Email invalid for id=" + id + ": " + email;
			logger.info(msg);
			throw new InvalidEmailException(msg);
		}
		return email;
	}

	private String[] getNames(String email) {
		String[] split = email.split("@");
		if (split.length == 1) {
			return new String[] { "", email };
		}
		return split;
	}

}
