var lastPwd = "";

$(document).ready(function() {
    var passwordInput = $('input[type=password]');

    var check = function() {
        var pwd = passwordInput.val();
        var meter = document.getElementById('meter');

        if (pwd.length < 4) {
            $('#score').empty().append("Score: 0");
            $('#entropy').empty().append("Entropy: 0");
            $('#cracktime_s').empty().append("Crack time (s): 0");
            $('#cracktime').empty().append("Crack time: instant");
            meter.style.width = "0px";
            return;
        }

        if (pwd == lastPwd) {
            return
        }

        var jqxhr = $.ajax({
            url: '/',
            type: 'post',
            data: { 'password': pwd },
            dataType: 'json'
        }).done(function(data) {
            console.log(data.entropy)
            $('#score').empty().append("Score: " + data.score);
            $('#entropy').empty().append("Entropy: " + data.entropy.toFixed(2));
            $('#cracktime_s').empty().append("Crack time (s): " + data.crack_time_s.toFixed(2));
            $('#cracktime').empty().append("Crack time: " + data.crack_time);
            meter.style.width = data.score * 10 + "%";

        }).fail(function(xhr, err) {
            console.log("readyState: " + xhr.readyState + "\nstatus: " + xhr.status);
            console.log("responseText: "+xhr.responseText);
        })
        lastPwd = pwd;
    }

    setInterval(check, 400);

    passwordInput.focus(function() {
        $('#pwd_info').show();
    }).blur(function() {
        $('#pwd_info').hide();
    });

});
