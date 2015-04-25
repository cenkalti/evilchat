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
            $scope.name = null;
            $scope.loggedIn = false;
            delete localStorage.name;
        }
        $scope.newWindow = function(peer) {
            var w = {
                thread: guid(),
                peer: peer
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
                    peer: message.from
                };
                if (!$scope.windowIds[message.thread]) {
                    $scope.windows.push(w);
                    $scope.windowIds[w.thread] = w;
                }
            })
        });
        $scope.$on("presence", function(event, message) {
            console.log("adding", message);
            $scope.$apply(function() {
                $scope.contacts[message.user] = {
                    name: message.user
                };
            });
        });
    })
    .controller('WindowController', function($scope, sock) {
        $scope.thread = $scope.window.thread;
        $scope.peer = $scope.window.peer;
        $scope.text = "";
        $scope.messages = [];
        $scope.messageIds = {};
        $scope.send = function() {
            var text = $scope.text.trim();
            if (!text) return;
            $scope.text = "";
            var message = {
                id: guid(),
                body: text
            };
            $scope.messageIds[message.id] = message;
            $scope.messages.push(message);
            sock.get().send(JSON.stringify({
                type: "chat",
                thread: $scope.thread,
                id: message.id,
                from: $scope.name,
                to: $scope.peer,
                body: text
            }));
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
    .directive('scrollBottom', function() {
        return function(scope, element, attrs) {
            element = element[0];
            window.messages = element;
            element.addEventListener('DOMNodeInserted', function(event) {
                if (event.target.tagName == "LI") {
                    console.log("scroll height", element.scrollHeight);
                    element.scrollTop = element.scrollHeight;
                }
            }, false);
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
