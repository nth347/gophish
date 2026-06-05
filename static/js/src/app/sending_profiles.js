var profiles = []

// Sample HTTP request bodies that the user can insert into the body field. The
// keys match the <option> values in the "Insert a sample body" dropdown.
var httpBodySamples = {
    basic: `{
    "to": "{{.To}}",
    "from": "{{.From}}",
    "subject": {{.Subject | json}},
    "html": {{.HTML | json}},
    "text": {{.Text | json}}
}`,
    attachments: `{
    "to": "{{.To}}",
    "from": "{{.From}}",
    "subject": {{.Subject | json}},
    "html": {{.HTML | json}},
    "text": {{.Text | json}},
    "attachments": [{{range $i, $a := .Attachments}}{{if $i}},{{end}}{
        "content": {{$a.Content | json}},
        "filename": {{$a.Filename | json}},
        "type": {{$a.Type | json}},
        "disposition": "attachment"
    }{{end}}]
}`,
    recipients: `{
    "to": {{.Recipients | json}},
    "from": { "address": "{{.From}}", "name": "{{.FromName}}" },
    "subject": {{.Subject | json}},
    "html": {{.HTML | json}},
    "text": {{.Text | json}}
}`
}

// insertHttpSample fills the HTTP body field with the selected sample template,
// confirming first if the field already has content.
function insertHttpSample() {
    var key = $("#http_body_sample").val()
    if (!key || !httpBodySamples[key]) {
        return
    }
    if ($("#http_body").val().trim() !== "" &&
        !confirm("Replace the current request body with the selected sample?")) {
        return
    }
    $("#http_body").val(httpBodySamples[key])
}

// toggleInterface shows the SMTP or HTTP specific fields depending on the
// currently selected interface type.
function toggleInterface() {
    if ($("#interface_type").val() === "HTTP") {
        $("#smtp_fields").hide()
        $("#http_fields").show()
    } else {
        $("#http_fields").hide()
        $("#smtp_fields").show()
    }
}

// Attempts to send a test email by POSTing to /campaigns/
function sendTestEmail() {
    var headers = [];
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        headers.push({
            key: unescapeHtml(header[0]),
            value: unescapeHtml(header[1]),
        })
    })
    var test_email_request = {
        template: {},
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        url: '',
        smtp: {
            interface_type: $("#interface_type").val(),
            from_address: $("#from").val(),
            host: $("#host").val(),
            username: $("#username").val(),
            password: $("#password").val(),
            ignore_cert_errors: $("#ignore_cert_errors").prop("checked"),
            headers: headers,
            http_method: $("#http_method").val(),
            http_url: $("#http_url").val(),
            http_headers: $("#http_headers").val(),
            http_content_type: $("#http_content_type").val(),
            http_body: $("#http_body").val(),
            http_batch_size: parseInt($("#http_batch_size").val(), 10) || 0,
            http_rate_per_second: parseInt($("#http_rate_per_second").val(), 10) || 0,
            http_rate_per_minute: parseInt($("#http_rate_per_minute").val(), 10) || 0,
            http_rate_per_hour: parseInt($("#http_rate_per_hour").val(), 10) || 0,
        }
    }
    btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    // Send the test email
    api.send_test_email(test_email_request)
        .success(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-success\">\
	    <i class=\"fa fa-check-circle\"></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append("<div style=\"text-align:center\" class=\"alert alert-danger\">\
	    <i class=\"fa fa-exclamation-circle\"></i> " + escapeHtml(data.responseJSON.message) + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

// Save attempts to POST to /smtp/
function save(idx) {
    var profile = {
        headers: []
    }
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        profile.headers.push({
            key: unescapeHtml(header[0]),
            value: unescapeHtml(header[1]),
        })
    })
    profile.name = $("#name").val()
    profile.interface_type = $("#interface_type").val()
    profile.from_address = $("#from").val()
    profile.host = $("#host").val()
    profile.username = $("#username").val()
    profile.password = $("#password").val()
    profile.ignore_cert_errors = $("#ignore_cert_errors").prop("checked")
    profile.http_method = $("#http_method").val()
    profile.http_url = $("#http_url").val()
    profile.http_headers = $("#http_headers").val()
    profile.http_content_type = $("#http_content_type").val()
    profile.http_body = $("#http_body").val()
    profile.http_batch_size = parseInt($("#http_batch_size").val(), 10) || 0
    profile.http_rate_per_second = parseInt($("#http_rate_per_second").val(), 10) || 0
    profile.http_rate_per_minute = parseInt($("#http_rate_per_minute").val(), 10) || 0
    profile.http_rate_per_hour = parseInt($("#http_rate_per_hour").val(), 10) || 0
    if (idx != -1) {
        profile.id = profiles[idx].id
        api.SMTPId.put(profile)
            .success(function (data) {
                successFlash("Profile edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    } else {
        // Submit the profile
        api.SMTP.post(profile)
            .success(function (data) {
                successFlash("Profile added successfully!")
                load()
                dismiss()
            })
            .error(function (data) {
                modalError(data.responseJSON.message)
            })
    }
}

function dismiss() {
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#interface_type").val("SMTP")
    $("#from").val("")
    $("#host").val("")
    $("#username").val("")
    $("#password").val("")
    $("#ignore_cert_errors").prop("checked", true)
    $("#http_method").val("POST")
    $("#http_url").val("")
    $("#http_headers").val("")
    $("#http_content_type").val("application/json")
    $("#http_body").val("")
    $("#http_body_sample").val("")
    $("#http_batch_size").val("1")
    $("#http_rate_per_second").val("0")
    $("#http_rate_per_minute").val("0")
    $("#http_rate_per_hour").val("0")
    $("#headersTable").dataTable().DataTable().clear().draw()
    toggleInterface()
    $("#modal").modal('hide')
}

var dismissSendTestEmailModal = function () {
    $("#sendTestEmailModal\\.flashes").empty()
    $("#sendTestModalSubmit").html("<i class='fa fa-envelope'></i> Send")
}


var deleteProfile = function (idx) {
    Swal.fire({
        title: "Are you sure?",
        text: "This will delete the sending profile. This can't be undone!",
        type: "warning",
        animation: false,
        showCancelButton: true,
        confirmButtonText: "Delete " + escapeHtml(profiles[idx].name),
        confirmButtonColor: "#428bca",
        reverseButtons: true,
        allowOutsideClick: false,
        preConfirm: function () {
            return new Promise(function (resolve, reject) {
                api.SMTPId.delete(profiles[idx].id)
                    .success(function (msg) {
                        resolve()
                    })
                    .error(function (data) {
                        reject(data.responseJSON.message)
                    })
            })
        }
    }).then(function (result) {
        if (result.value){
            Swal.fire(
                'Sending Profile Deleted!',
                'This sending profile has been deleted!',
                'success'
            );
        }
        $('button:contains("OK")').on('click', function () {
            location.reload()
        })
    })
}

function edit(idx) {
    headers = $("#headersTable").dataTable({
        destroy: true, // Destroy any other instantiated table - http://datatables.net/manual/tech-notes/3#destroy
        columnDefs: [{
            orderable: false,
            targets: "no-sort"
        }]
    })

    $("#modalSubmit").unbind('click').click(function () {
        save(idx)
    })
    var profile = {}
    if (idx != -1) {
        $("#profileModalLabel").text("Edit Sending Profile")
        profile = profiles[idx]
        $("#name").val(profile.name)
        $("#interface_type").val(profile.interface_type)
        $("#from").val(profile.from_address)
        $("#host").val(profile.host)
        $("#username").val(profile.username)
        $("#password").val(profile.password)
        $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
        $("#http_method").val(profile.http_method || "POST")
        $("#http_url").val(profile.http_url)
        $("#http_headers").val(profile.http_headers)
        $("#http_content_type").val(profile.http_content_type)
        $("#http_body").val(profile.http_body)
        $("#http_batch_size").val(profile.http_batch_size || 1)
        $("#http_rate_per_second").val(profile.http_rate_per_second || 0)
        $("#http_rate_per_minute").val(profile.http_rate_per_minute || 0)
        $("#http_rate_per_hour").val(profile.http_rate_per_hour || 0)
        $.each(profile.headers, function (i, record) {
            addCustomHeader(record.key, record.value)
        });
    } else {
        $("#profileModalLabel").text("New Sending Profile")
    }
    toggleInterface()
}

function copy(idx) {
    $("#modalSubmit").unbind('click').click(function () {
        save(-1)
    })
    var profile = {}
    profile = profiles[idx]
    $("#name").val("Copy of " + profile.name)
    $("#interface_type").val(profile.interface_type)
    $("#from").val(profile.from_address)
    $("#host").val(profile.host)
    $("#username").val(profile.username)
    $("#password").val(profile.password)
    $("#ignore_cert_errors").prop("checked", profile.ignore_cert_errors)
    $("#http_method").val(profile.http_method || "POST")
    $("#http_url").val(profile.http_url)
    $("#http_headers").val(profile.http_headers)
    $("#http_content_type").val(profile.http_content_type)
    $("#http_body").val(profile.http_body)
    $("#http_batch_size").val(profile.http_batch_size || 1)
    $("#http_rate_per_second").val(profile.http_rate_per_second || 0)
    $("#http_rate_per_minute").val(profile.http_rate_per_minute || 0)
    $("#http_rate_per_hour").val(profile.http_rate_per_hour || 0)
    toggleInterface()
}

function load() {
    $("#profileTable").hide()
    $("#emptyMessage").hide()
    $("#loading").show()
    api.SMTP.get()
        .success(function (ss) {
            profiles = ss
            $("#loading").hide()
            if (profiles.length > 0) {
                $("#profileTable").show()
                profileTable = $("#profileTable").DataTable({
                    destroy: true,
                    columnDefs: [{
                        orderable: false,
                        targets: "no-sort"
                    }]
                });
                profileTable.clear()
                profileRows = []
                $.each(profiles, function (i, profile) {
                    profileRows.push([
                        escapeHtml(profile.name),
                        profile.interface_type,
                        moment(profile.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'><span data-toggle='modal' data-backdrop='static' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Profile' onclick='edit(" + i + ")'>\
                    <i class='fa fa-pencil'></i>\
                    </button></span>\
		    <span data-toggle='modal' data-target='#modal'><button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Profile' onclick='copy(" + i + ")'>\
                    <i class='fa fa-copy'></i>\
                    </button></span>\
                    <button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Profile' onclick='deleteProfile(" + i + ")'>\
                    <i class='fa fa-trash-o'></i>\
                    </button></div>"
                    ])
                })
                profileTable.rows.add(profileRows).draw()
                $('[data-toggle="tooltip"]').tooltip()
            } else {
                $("#emptyMessage").show()
            }
        })
        .error(function () {
            $("#loading").hide()
            errorFlash("Error fetching profiles")
        })
}

function addCustomHeader(header, value) {
    // Create new data row.
    var newRow = [
        escapeHtml(header),
        escapeHtml(value),
        '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>'
    ];

    // Check table to see if header already exists.
    var headersTable = headers.DataTable();
    var existingRowIndex = headersTable
        .column(0) // Email column has index of 2
        .data()
        .indexOf(escapeHtml(header));

    // Update or add new row as necessary.
    if (existingRowIndex >= 0) {
        headersTable
            .row(existingRowIndex, {
                order: "index"
            })
            .data(newRow);
    } else {
        headersTable.row.add(newRow);
    }
    headersTable.draw();
}

$(document).ready(function () {
    // Setup multiple modals
    // Code based on http://miles-by-motorcycle.com/static/bootstrap-modal/index.html
    $('.modal').on('hidden.bs.modal', function (event) {
        $(this).removeClass('fv-modal-stack');
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1);
    });
    $('.modal').on('shown.bs.modal', function (event) {
        // Keep track of the number of open modals
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') {
            $('body').data('fv_open_modals', 0);
        }
        // if the z-index of this modal has been set, ignore.
        if ($(this).hasClass('fv-modal-stack')) {
            return;
        }
        $(this).addClass('fv-modal-stack');
        // Increment the number of open modals
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1);
        // Setup the appropriate z-index
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')));
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack');
    });
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document)
            .off('focusin.bs.modal') // guard against infinite focus loop
            .on('focusin.bs.modal', $.proxy(function (e) {
                if (
                    this.$element[0] !== e.target && !this.$element.has(e.target).length
                    // CKEditor compatibility fix start.
                    &&
                    !$(e.target).closest('.cke_dialog, .cke').length
                    // CKEditor compatibility fix end.
                ) {
                    this.$element.trigger('focus');
                }
            }, this));
    };
    // Scrollbar fix - https://stackoverflow.com/questions/19305821/multiple-modals-overlay
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open');
    });
    $('#modal').on('hidden.bs.modal', function (event) {
        dismiss()
    });
    $("#sendTestEmailModal").on("hidden.bs.modal", function (event) {
        dismissSendTestEmailModal()
    })
    // Toggle SMTP/HTTP fields when the interface type changes
    $("#interface_type").on('change', function () {
        toggleInterface()
    })
    // Insert the selected sample HTTP body
    $("#insertHttpSample").on('click', function () {
        insertHttpSample()
    })
    // Code to deal with custom email headers
    $("#addCustomHeader").on('click', function () {
        headerKey = $("#headerKey").val();
        headerValue = $("#headerValue").val();

        if (headerKey == "" || headerValue == "") {
            return false;
        }
        addCustomHeader(headerKey, headerValue);
        // Reset user input.
        $("#headerKey").val('');
        $("#headerValue").val('');
        $("#headerKey").focus();
        return false;
    });
    // Handle Deletion
    $("#headersTable").on("click", "span>i.fa-trash-o", function () {
        headers.DataTable()
            .row($(this).parents('tr'))
            .remove()
            .draw();
    });
    load()
})
