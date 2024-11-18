"""Make some requests to OpenAI's chatbot"""

import time
import os 
import flask
import sys

from flask import g

from playwright.sync_api import sync_playwright

PROFILE_DIR = "/tmp/playwright" if '--profile' not in sys.argv else sys.argv[sys.argv.index('--profile') + 1]
PORT = 5001 if '--port' not in sys.argv else int(sys.argv[sys.argv.index('--port') + 1])
APP = flask.Flask(__name__)
PLAY = sync_playwright().start()
BROWSER = PLAY.firefox.launch_persistent_context(
    user_data_dir=PROFILE_DIR,
    headless=False,
)
PAGE = BROWSER.new_page()

def get_input_box():
    """Find the input box in the updated ChatGPT interface."""
    # Updated selector for the new ChatGPT interface
    textarea = PAGE.query_selector("#prompt-textarea")
    return textarea

def is_logged_in():
    try:
        return get_input_box() is not None
    except AttributeError:
        return False

def send_message(message):
    box = get_input_box()
    box.click()
    box.fill(message)
    # Find and click the send button
    send_button = PAGE.query_selector("button[data-testid='send-button']")
    send_button.click()
    # Wait for response to complete
    PAGE.wait_for_selector(".result-streaming", state="detached", timeout=60000)

def get_last_message():
    """Get the latest message from the updated interface"""
    # Updated selector for the new ChatGPT interface
    messages = PAGE.query_selector_all("div[data-message-author-role='assistant']")
    if messages:
        return messages[-1].inner_text()
    return "No response received"

@APP.route("/chat", methods=["GET"])
def chat():
    message = flask.request.args.get("q")
    print("Sending message: ", message)
    send_message(message)
    response = get_last_message()
    print("Response: ", response)
    return response

def start_browser():
    PAGE.goto("https://chat.openai.com/")
    APP.run(port=PORT, threaded=False)
    if not is_logged_in():
        print("Please log in to OpenAI Chat")
        print("Press enter when you're done")
        input()
    else:
        print("Logged in")
        
if __name__ == "__main__":
    start_browser()