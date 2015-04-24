angular.module('x', [])
.controller('XController', function($scope, sockjs) {
    $scope.name = null;
    $scope.login = function () {
        $scope.name = prompt("Enter your name:");
        sockjs.send(JSON.stringify({
            type: "login",
            name: $scope.name
        }));
    }
    $scope.contacts = [
        {name: "cenk"},
        {name: "rauf"},
        {name: "ismigül"}];

    // this.addContact = function() {
    //   this.list.push({???});
    // };

    // this.archive = function() {
    //   var oldTodos = this.list;
    //   this.list = [];
    //   angular.forEach(oldTodos, function(todo) {
    //     if (!todo.done) this.list.push(todo);
    //   });
    // };
})
.factory('sockjs', function () {
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


// function login () {
//     name = prompt("Enter your name:");
//     sock.send(name);
// }

// function send (el) {
//     el = $(el);
//     console.log("sending message", el.val());
//     sock.send(JSON.stringify({
//         from: name,
//         to: el.data('to'),
//         body: el.val()
//     }));
//     el.val('');
// }

// $(function () {
//     $('.input-send').keypress(function () {
//         if(event.keyCode == 13) { // enter key
//             send(this);
//         }
//     });
// });
