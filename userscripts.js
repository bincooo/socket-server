// ==UserScript==
// @name         NewBing自动校验通讯
// @namespace    http://tampermonkey.net/
// @version      2024-01-05
// @description  try to take over the world!
// @author       You
// @match        https://www.bing.com/turing/captcha/challenge
// @match        https://challenges.cloudflare.com/cdn-cgi/challenge-platform/*
// @icon         https://www.google.com/s2/favicons?sz=64&domain=bing.com
// @grant        none

// @require      https://cdn.jsdelivr.net/npm/js-cookie@3.0.5/dist/js.cookie.min.js
// ==/UserScript==

(function() {
    'use strict';

    var { hash, origin } = window.location
    if (origin === "https://challenges.cloudflare.com") {
        init_child(window)
        return
    }

    if (origin !== "https://www.bing.com") {
        return
    }

    if (hash.length < 3 || !hash.startsWith('#ip=')) {
        return
    }

    init_sock(window, hash.substr(4));
})();

function uuidv4() {
    return "10000000-1000-4000-8000-100000000000".replace(/[018]/g, c => (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16));
}

function init_sock(window, ip) {
    window.document.querySelector('iframe').remove()
    // init page
    var console = document.createElement('div')
    console.style.minHeight = '500px'
    console.style.border = '1px solid'
    window.document.body.appendChild(console)

    var log = (...args) => {
        var buffer = ""
        for (let i = 0; i < args.length; i++) {
            var it = args[i]
            if (typeof it === "object") {
                buffer += JSON.stringify(it)
            } else {
                buffer += it.toString()
            }
        }
        var items = buffer.split("\n")
        for (let i = 0; i < items.length; i++) {
            let newline = window.document.createElement('div')
            newline.innerText = items[i]
            console.appendChild(newline)
        }
    }

    if (!window['WebSocket']) {
        // alert('该浏览器不支持WebSocket!')
        log('该浏览器不支持WebSocket!')
        return
    }

    log('🎉🎉🎉 Welcome new bing! 🎉🎉🎉')

    var iframe = undefined

    var conn = new WebSocket("ws://" + ip + "/ws?"+ uuidv4());
    conn.onclose = function (evt) {
        // alert('sock close!')
        log('sock close!')
    };
    conn.onmessage = function (evt) {
        log("server event: " + evt.data)
        if (evt.data === "delete") {
            iframe.remove()
            iframe = undefined
            return
        }
        var cookies = JSON.parse(evt.data)
        for (let index in cookies) {
            var cookie = cookies[index]
            Cookies.set(cookie.Name, cookie.Value, { expires: 100, path: cookie.Path, domain: cookie.Domain })
        }
        conn.send("OK")
        if (!iframe) {
            iframe = document.createElement('iframe')
            // https://www.bing.com/turing/captcha/challenge#ip=127.0.0.1:8080
            iframe.src = "https://www.bing.com/turing/captcha/challenge"
            // iframe.style.display = "none"
            window.document.body.appendChild(iframe)
        }
        iframe.contentWindow.location.reload()
    };

    event(window, function(data) {
        log(data)
        conn.send(data.message)
    })
}

function init_child(window) {
    setTimeout(() => {
        postMessage(window.top, {
            type: 'message',
            message: "ping"
        })
        //
    }, 1000)

    var timer
    timer = setInterval(() => {
        var success = window.document.querySelector('#success')
        if (success && !success.style.display?.includes('none')) {
            clearInterval(timer)
            postMessage(window.top, {
                type: 'message',
                message: "success"
            })
            return
        }

        postMessage(window.top, {
            type: 'message',
            message: "trying..."
        })

        var checkbox = window.document.querySelector('#challenge-stage label.ctp-checkbox-label > input[type=checkbox]')
        if (checkbox) {
            postMessage(window.top, {
                type: 'message',
                message: "trying click checkbox."
            })
            checkbox.click()
        } else {
            postMessage(window.top, {
                type: 'message',
                message: "not find input[type=checkbox]"
            })
        }
    }, 1000)
}

function postMessage(window, data) {
    window.postMessage(data, "*")
}

function event(window, callback) {
    window.addEventListener('message', e => {
        if (e.data?.type === "message") {
            callback(e.data)
        }
    }, false)
}