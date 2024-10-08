#! /bin/env python3
"""
Functions and scripts to sync OIDC identities on user accounts
"""

import os
import json
import time
import logging
import requests
from urllib import parse

logger = logging.getLogger(__name__)

if __name__ == '__main__':
    """
    sync OIDC identities on user accounts
    """
    try:
        verbose = os.environ.get("VERBOSE")
        if "True" == verbose:
            logging.basicConfig(level=logging.DEBUG)
            logger.info("Verbose mode: setting log to debug.")
        
        iam_grant_type = os.environ.get("IAM_GRANT_TYPE")
        iam_server = os.environ.get(
            "IAM_TOKEN_ENDPOINT", "https://cms-auth.web.cern.ch/token")
        iam_client_id = os.environ.get("IAM_CLIENT_ID")
        iam_client_secret = os.environ.get("IAM_CLIENT_SECRET")
        iam_refresh_token = os.environ.get("IAM_REFRESH_TOKEN")
        audience = os.environ.get("IAM_VK_AUD")
        output_file = os.environ.get("TOKEN_PATH", "/opt/interlink/token")
    except Exception:
        logger.exception()
        exit(1)

    while True:
        try:
            with open(output_file+"-refresh", "r") as text_file:
                rt = text_file.readline()
            if rt != "": 
                iam_refresh_token = rt
        except:
            logger.info("No cache for refresh token, starting from ENV value")
    
        logger.info("Current refresh token: %s", iam_refresh_token)
        token = None

        if iam_grant_type == "client_credentials": 
            try:
                request_data = {
                    "audience": audience,
                    "grant_type": iam_grant_type,
                    "client_id" : iam_client_id,
                    "client_secret": iam_client_secret
                    #"scope": "openid profile email address phone offline_access"
                }

                from requests.auth import HTTPBasicAuth
                auth = HTTPBasicAuth(iam_client_id, iam_client_secret)
                headers = {'Content-Type': 'application/x-www-form-urlencoded'}
                r = requests.post(iam_server, data=request_data, auth=auth, headers=headers)
                logger.debug("Raw response text: %s", r.text)
                try:
                    response = json.loads(r.text)
                except:
                    try:
                        response = dict(parse.parse_qsl(r.text)) 
                        logger.debug("Response text parsed: %s", response)
                    except:
                        exit(1)
                        

                logger.debug("iam_client_id %s iam_client_secret %s response %s", iam_client_id, iam_client_secret, response)

                token = response['access_token']
                try:
                    refresh_token = response['refresh_token']
                except:
                    refresh_token = iam_refresh_token


                logger.info("Token retrieved")

                ## TODO: collect new refresh token and store it somewhere
                with open(output_file+"-refresh", "w") as text_file:
                    text_file.write(refresh_token)

                with open(output_file, "w") as text_file:
                    text_file.write(token)

                logger.info(f"Refresh token written in {output_file+'-refresh'}")

            except Exception as e:
                logger.warning("ERROR oidc get token: {}".format(e), exc_info=True)
                logger.warning("Response if available: %s", response)
            
        elif iam_grant_type == "authorization_code":

            try:
                request_data = {
                    "audience": audience,
                    "grant_type": "refresh_token",
                    "refresh_token": iam_refresh_token,
                    #"scope": "openid profile email address phone offline_access"
                }

                from requests.auth import HTTPBasicAuth
                auth = HTTPBasicAuth(iam_client_id, iam_client_secret)

                r = requests.post(iam_server, data=request_data, auth=auth)
                print(r.text)
                try:
                    response = json.loads(r.text)
                except:
                    try:
                        response = dict(parse.parse_qsl(r.text)) 
                        logger.debug(response)
                    except:
                        exit(1)
                        
                logger.debug("iam_client_id %s iam_client_secret %s response %s", iam_client_id, iam_client_secret, response)

                token = response['access_token']
                try:
                    refresh_token = response['refresh_token']
                except:
                    refresh_token = iam_refresh_token


                logger.info("Token retrieved")

                ## TODO: collect new refresh token and store it somewhere
                with open(output_file+"-refresh", "w") as text_file:
                    text_file.write(refresh_token)


                with open(output_file, "w") as text_file:
                    text_file.write(token)

                logger.info(f"Refresh token written in {output_file+'-refresh'}")

            except Exception as e:
                logger.warning("ERROR oidc get token: {}".format(e), exc_info=True)
                logger.warning("Response if available: %s", response)
        else:
            logger.error(f"Invalid grant type {iam_grant_type}", exc_info=True)
            exit(1)
        time.sleep(200)
