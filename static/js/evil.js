angular.module('main', [])
    .controller('MainController', function($scope, sock) {
        $scope.loggedIn = false;
        $scope.name = null;
        $scope.contacts = {};
        $scope.windows = [];
        $scope.windowIds = {};
        $scope.login = function() {
            $scope.name = prompt("Enter your name:");
            localStorage.name = $scope.name;
            sock.get().send(JSON.stringify({
                type: "login",
                name: $scope.name
            }));
            $scope.loggedIn = true;
        }
        $scope.logout = function() {
            delete localStorage.name;
            location.reload();
        }
        $scope.newWindow = function(contact) {
            var w = {
                thread: guid(),
                contact: contact
            };
            $scope.windows.push(w);
            $scope.windowIds[w.thread] = w;
        }
        $scope.closeWindow = function(thread) {
            var w = $scope.windowIds[thread];
            if (!w) return;
            var i = $scope.windows.indexOf(w);
            if (i == -1) return;
            $scope.windows.splice(i, 1);
            delete $scope.windowIds[thread];
        };
        if (localStorage.name) {
            $scope.name = localStorage.name;
            $scope.loggedIn = true;
        }
        $scope.$on("thread", function(event, message) {
            $scope.$apply(function() {
                var w = {
                    thread: message.thread,
                    contact: {
                        name: message.from
                    }
                };
                if (!$scope.windowIds[message.thread]) {
                    $scope.windows.push(w);
                    $scope.windowIds[w.thread] = w;
                }
            })
        });
        $scope.$on("presence", function(event, message) {
            $scope.$apply(function() {
                switch (message.status) {
                    case "online":
                        $scope.contacts[message.name] = {
                            name: message.name
                        };
                        break;
                    case "offline":
                        delete $scope.contacts[message.name];
                        break
                    default:
                        console.log("unknown presence status", message);
                }
            });
        });
    })
    .controller('WindowController', function($scope, sock) {
        $scope.thread = $scope.window.thread;
        $scope.contact = $scope.window.contact;
        $scope.text = "";
        $scope.messages = [];
        $scope.messageIds = {};
        $scope.send = function() {
            var text = $scope.text.trim();
            if (!text) return;
            $scope.text = "";
            var message = {
                type: "chat",
                thread: $scope.thread,
                id: guid(),
                from: $scope.name,
                to: $scope.contact.name,
                body: text
            };
            $scope.messageIds[message.id] = message;
            $scope.messages.push(message);
            sock.get().send(JSON.stringify(message));
        };
        $scope.$on("chat." + $scope.thread, function(event, message) {
            $scope.$apply(function() {
                if ($scope.messageIds[message.id]) {
                    console.log("ignoring duplicate message", message);
                    return;
                }
                $scope.messageIds[message.id] = message;
                $scope.messages.push(message);
            })
        });
    })
    .directive('enter', function() {
        return function(scope, element, attrs) {
            element.bind("keypress", function(event) {
                if (event.which === 13) {
                    scope.$apply(function() {
                        scope.$eval(attrs.enter);
                    });
                    event.preventDefault();
                }
            });
        }
    })
    .directive('noScrollParent', function() {
        return function(scope, element, attrs) {
            element[0].onmousewheel = function(e) {
                e.preventDefault();
                this.scrollTop += (e.wheelDelta * -1);
            }
        }
    })
    .directive('scrollBottom', function() {
        return function(scope, element, attrs) {
            element = element[0];
            var observer = new MutationObserver(function(mutations) {
                mutations.forEach(function(mutation) {
                    if (mutation.addedNodes) {
                        for (var i = 0; i < mutation.addedNodes.length; i++) {
                            var node = mutation.addedNodes[i];
                            if (node.nodeName == "LI") {
                                element.scrollTop = element.scrollHeight;
                            }
                        }
                    }
                });
            });
            observer.observe(element, {
                childList: true
            });
        }
    })
    .factory('sock', function($rootScope) {
        var sock = null;
        var retryInterval = null;

        function newSocket() {
            sock = new SockJS('/sockjs/sock');
            sock.onopen = function() {
                clearInterval(retryInterval);
                console.log('opened sockjs session');
                if (localStorage.name) {
                    sock.send(JSON.stringify({
                        type: "login",
                        name: localStorage.name
                    }));
                }
            };
            sock.onmessage = function(e) {
                var message = JSON.parse(e.data);
                console.log('received message', message);
                switch (message.type) {
                    case "chat":
                        $rootScope.$broadcast('thread', message);
                        $rootScope.$broadcast('chat.' + message.thread, message);
                        break;
                    case "presence":
                        $rootScope.$broadcast('presence', message);
                        break;
                    default:
                        console.log("unknown message type", message.type);
                }
            };
            sock.onclose = function() {
                sock = null;
                console.log('closed sockjs session');
                retryInterval = setTimeout(newSocket, 2000)
            };
        }
        newSocket();
        return {
            get: function() {
                return sock;
            }
        };
    });

function guid() {
    function _p8(s) {
        var p = (Math.random().toString(16) + "000000000").substr(2, 8);
        return s ? "-" + p.substr(0, 4) + "-" + p.substr(4, 4) : p;
    }
    return _p8() + _p8(true) + _p8(true) + _p8();
}
