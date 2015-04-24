angular.module('main', [])
    .controller('MainController', function($scope, sock) {
        $scope.loggedIn = false;
        $scope.name = null;
        $scope.contacts = [{
            name: "cenk"
        }, {
            name: "rauf"
        }, {
            name: "ismigül"
        }];
        $scope.windows = [];
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
        $scope.newWindow = function(to) {
            $scope.windows.push({
                id: guid(),
                to: to
            });
        }
        if (localStorage.name) {
            $scope.name = localStorage.name;
            $scope.loggedIn = true;
        }
    })
    .controller('WindowController', function($scope, sock) {
        $scope.id = $scope.window.id;
        $scope.to = $scope.window.to;
        $scope.text = "";
        $scope.messages = [];
        $scope.send = function() {
            var text = $scope.text;
            $scope.text = "";
            $scope.messages.push({
                body: text
            });
            sock.get().send(JSON.stringify({
                type: "chat",
                id: $scope.id,
                from: $scope.name,
                to: $scope.to,
                body: text
            }));
        };
        $scope.$on("chat." + $scope.id, function(event, message) {
            console.log("message", message);
            $scope.$apply(function() {
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
                        $rootScope.$broadcast('chat.' + message.id, message);
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
