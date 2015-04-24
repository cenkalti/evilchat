angular.module('main', [])
    .controller('MainController', function($scope, sockjs) {
        $scope.name = null;
        $scope.contacts = [{
            name: "cenk"
        }, {
            name: "rauf"
        }, {
            name: "ismigül"
        }];
        $scope.windows = [{
            to: "cenk",
        }];
        $scope.login = function() {
            $scope.name = prompt("Enter your name:");
            sockjs.send(JSON.stringify({
                type: "login",
                name: $scope.name
            }));
        }
        $scope.newWindow = function(to) {
            $scope.windows.push({
                to: to
            });
        }
    })
    .controller('WindowController', function($scope, sockjs) {
        $scope.to = $scope.window.to;
        $scope.text = "";
        $scope.messages = [];
        $scope.send = function() {
            var text = $scope.text;
            $scope.text = "";
            $scope.messages.push({
                body: text
            });
            sockjs.send(JSON.stringify({
                type: "chat",
                from: $scope.name,
                to: $scope.to,
                body: text
            }));
        };
    })
    .directive('enter', function() {
        return {
            link: function(scope, element, attrs) {
                element.bind("keydown keypress", function(event) {
                    if (event.which === 13) {
                        scope.$apply(function() {
                            scope.$eval(attrs.enter);
                        });
                        event.preventDefault();
                    }
                });
            }
        };
    })
    .factory('sockjs', function() {
        var sock = new SockJS('/sockjs/sock');
        sock.onopen = function() {
            console.log('opened sockjs session');
        };
        sock.onmessage = function(e) {
            console.log('received message', e.data);
        };
        sock.onclose = function() {
            console.log('closed sockjs session');
        };
        return sock;
    });
