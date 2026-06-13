var profiles = []
var currentProfileId = -1

// Sample HTTP request bodies
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

function insertHttpSample() {
    var key = $("#http_body_sample").val()
    if (!key || !httpBodySamples[key]) return
    if ($("#http_body").val().trim() !== "" &&
        !confirm("Replace the current request body with the selected sample?")) return
    $("#http_body").val(httpBodySamples[key])
}

// toggleInterface shows only the fields relevant to the selected type.
function toggleInterface() {
    var iface = $("#interface_type").val()
    $("#smtp_fields").hide()
    $("#gmail_fields").hide()
    $("#outlook_fields").hide()
    $("#http_fields").hide()
    // Cert errors only apply to raw SMTP / HTTP transports.
    $("#ignore_cert_errors_row").toggle(iface === "SMTP" || iface === "HTTP")
    if (iface === "SMTP")            $("#smtp_fields").show()
    else if (iface === "Gmail")      $("#gmail_fields").show()
    else if (iface === "OutlookOAuth2") $("#outlook_fields").show()
    else if (iface === "HTTP")       $("#http_fields").show()
}

function setOutlookAuthStatus(authenticated) {
    if (authenticated) {
        $("#outlook_auth_status")
            .removeClass("alert-warning alert-danger")
            .addClass("alert-success")
            .html("<i class='fa fa-check-circle'></i> Authenticated &mdash; token is stored. The textarea above is intentionally blank; paste new JSON here only if you want to replace the token.")
        $("#outlook_token_cache_input")
            .attr("placeholder", "Token already stored. Paste new token_cache.json content here only to replace it.")
    } else {
        $("#outlook_auth_status")
            .removeClass("alert-success alert-danger")
            .addClass("alert-warning")
            .html("<i class='fa fa-exclamation-triangle'></i> Not authenticated &mdash; run the script and paste token_cache.json content above, then save.")
        $("#outlook_token_cache_input")
            .attr("placeholder", '{"AccessToken": {...}, "RefreshToken": {...}, ...}')
    }
}

// sendTestEmail sends a test email using the current form values.
function sendTestEmail() {
    var headers = []
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        headers.push({ key: unescapeHtml(header[0]), value: unescapeHtml(header[1]) })
    })
    var iface = $("#interface_type").val()
    var test_email_request = {
        template: {
            subject: $("#test_subject").val(),
            html:    $("#test_html").val(),
            text:    $("#test_text").val(),
        },
        first_name: $("input[name=to_first_name]").val(),
        last_name: $("input[name=to_last_name]").val(),
        email: $("input[name=to_email]").val(),
        position: $("input[name=to_position]").val(),
        url: '',
        smtp: {
            name: $("#name").val(),
            interface_type: iface,
            from_address: $("#from").val(),
            host: $("#host").val(),
            username: $("#username").val(),
            password: iface === "Gmail" ? $("#gmail_password").val() : $("#password").val(),
            ignore_cert_errors: $("#ignore_cert_errors").prop("checked"),
            headers: headers,
            outlook_client_id: $("#outlook_client_id").val(),
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
    var btnHtml = $("#sendTestModalSubmit").html()
    $("#sendTestModalSubmit").html('<i class="fa fa-spinner fa-spin"></i> Sending')
    api.send_test_email(test_email_request)
        .success(function () {
            $("#sendTestEmailModal\\.flashes").empty().append(
                "<div style='text-align:center' class='alert alert-success'>" +
                "<i class='fa fa-check-circle'></i> Email Sent!</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
        .error(function (data) {
            $("#sendTestEmailModal\\.flashes").empty().append(
                "<div style='text-align:center' class='alert alert-danger'>" +
                "<i class='fa fa-exclamation-circle'></i> " +
                escapeHtml(data.responseJSON.message) + "</div>")
            $("#sendTestModalSubmit").html(btnHtml)
        })
}

// save POSTs or PUTs the current form values.
function save(idx) {
    var profile = { headers: [] }
    $.each($("#headersTable").DataTable().rows().data(), function (i, header) {
        profile.headers.push({ key: unescapeHtml(header[0]), value: unescapeHtml(header[1]) })
    })
    var iface = $("#interface_type").val()
    profile.name           = $("#name").val()
    profile.interface_type = iface
    profile.from_address   = $("#from").val()
    profile.ignore_cert_errors = $("#ignore_cert_errors").prop("checked")

    if (iface === "Gmail") {
        profile.password = $("#gmail_password").val()
    } else if (iface === "OutlookOAuth2") {
        profile.outlook_client_id = $("#outlook_client_id").val()
        profile.outlook_token_cache_input = $("#outlook_token_cache_input").val()
    } else if (iface === "SMTP") {
        profile.host     = $("#host").val()
        profile.username = $("#username").val()
        profile.password = $("#password").val()
    } else if (iface === "HTTP") {
        profile.host             = $("#host").val()
        profile.username         = $("#username").val()
        profile.password         = $("#password").val()
        profile.http_method      = $("#http_method").val()
        profile.http_url         = $("#http_url").val()
        profile.http_headers     = $("#http_headers").val()
        profile.http_content_type = $("#http_content_type").val()
        profile.http_body        = $("#http_body").val()
        profile.http_batch_size  = parseInt($("#http_batch_size").val(), 10) || 0
        profile.http_rate_per_second = parseInt($("#http_rate_per_second").val(), 10) || 0
        profile.http_rate_per_minute = parseInt($("#http_rate_per_minute").val(), 10) || 0
        profile.http_rate_per_hour   = parseInt($("#http_rate_per_hour").val(), 10) || 0
    }

    if (idx != -1) {
        profile.id = profiles[idx].id
        api.SMTPId.put(profile)
            .success(function (data) {
                successFlash("Profile edited successfully!")
                load()
                dismiss()
            })
            .error(function (data) { modalError(data.responseJSON.message) })
    } else {
        api.SMTP.post(profile)
            .success(function (data) {
                successFlash("Profile added successfully!")
                load()
                dismiss()
            })
            .error(function (data) { modalError(data.responseJSON.message) })
    }
}

function dismiss() {
    currentProfileId = -1
    $("#modal\\.flashes").empty()
    $("#name").val("")
    $("#interface_type").val("SMTP")
    $("#from").val("")
    $("#host").val("")
    $("#username").val("")
    $("#password").val("")
    $("#gmail_password").val("")
    $("#outlook_client_id").val("")
    $("#outlook_token_cache_input").val("")
    setOutlookAuthStatus(false)
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
                    .success(function () { resolve() })
                    .error(function (data) { reject(data.responseJSON.message) })
            })
        }
    }).then(function (result) {
        if (result.value) {
            Swal.fire('Sending Profile Deleted!', 'This sending profile has been deleted!', 'success')
        }
        $('button:contains("OK")').on('click', function () { location.reload() })
    })
}

function edit(idx) {
    headers = $("#headersTable").dataTable({
        destroy: true,
        columnDefs: [{ orderable: false, targets: "no-sort" }]
    })
    $("#modalSubmit").unbind('click').click(function () { save(idx) })

    if (idx != -1) {
        $("#profileModalLabel").text("Edit Sending Profile")
        var profile = profiles[idx]
        currentProfileId = profile.id
        $("#name").val(profile.name)
        $("#interface_type").val(profile.interface_type)
        $("#from").val(profile.from_address)
        $("#host").val(profile.host)
        $("#username").val(profile.username)
        $("#password").val(profile.password)
        $("#gmail_password").val(profile.password)
        $("#outlook_client_id").val(profile.outlook_client_id)
        $("#outlook_token_cache_input").val("")   // never pre-fill with stored token
        setOutlookAuthStatus(profile.outlook_authenticated)
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
        $.each(profile.headers, function (i, record) { addCustomHeader(record.key, record.value) })
    } else {
        currentProfileId = -1
        $("#profileModalLabel").text("New Sending Profile")
        setOutlookAuthStatus(false)
    }
    toggleInterface()
}

function copy(idx) {
    headers = $("#headersTable").dataTable({
        destroy: true,
        columnDefs: [{ orderable: false, targets: "no-sort" }]
    })
    $("#modalSubmit").unbind('click').click(function () { save(-1) })
    currentProfileId = -1
    var profile = profiles[idx]
    $("#name").val("Copy of " + profile.name)
    $("#interface_type").val(profile.interface_type)
    $("#from").val(profile.from_address)
    $("#host").val(profile.host)
    $("#username").val(profile.username)
    $("#password").val(profile.password)
    $("#gmail_password").val(profile.password)
    $("#outlook_client_id").val(profile.outlook_client_id)
    $("#outlook_token_cache_input").val("")   // require re-authentication for copies
    setOutlookAuthStatus(false)
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
    $.each(profile.headers, function (i, record) { addCustomHeader(record.key, record.value) })
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
                var profileTable = $("#profileTable").DataTable({
                    destroy: true,
                    columnDefs: [{ orderable: false, targets: "no-sort" }]
                })
                profileTable.clear()
                var profileRows = []
                $.each(profiles, function (i, profile) {
                    var ifaceLabel = profile.interface_type
                    if (ifaceLabel === "OutlookOAuth2") {
                        ifaceLabel = "Outlook OAuth2 " + (profile.outlook_authenticated ? "&#10003;" : "&#9888;")
                    } else if (ifaceLabel === "Gmail") {
                        ifaceLabel = "Gmail (App Password)"
                    }
                    profileRows.push([
                        escapeHtml(profile.name),
                        ifaceLabel,
                        moment(profile.modified_date).format('MMMM Do YYYY, h:mm:ss a'),
                        "<div class='pull-right'>" +
                        "<span data-toggle='modal' data-backdrop='static' data-target='#modal'>" +
                        "<button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Edit Profile' onclick='edit(" + i + ")'>" +
                        "<i class='fa fa-pencil'></i></button></span> " +
                        "<span data-toggle='modal' data-target='#modal'>" +
                        "<button class='btn btn-primary' data-toggle='tooltip' data-placement='left' title='Copy Profile' onclick='copy(" + i + ")'>" +
                        "<i class='fa fa-copy'></i></button></span> " +
                        "<button class='btn btn-danger' data-toggle='tooltip' data-placement='left' title='Delete Profile' onclick='deleteProfile(" + i + ")'>" +
                        "<i class='fa fa-trash-o'></i></button></div>"
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
    var newRow = [escapeHtml(header), escapeHtml(value), '<span style="cursor:pointer;"><i class="fa fa-trash-o"></i></span>']
    var headersTable = headers.DataTable()
    var existing = headersTable.column(0).data().indexOf(escapeHtml(header))
    if (existing >= 0) {
        headersTable.row(existing, { order: "index" }).data(newRow)
    } else {
        headersTable.row.add(newRow)
    }
    headersTable.draw()
}

$(document).ready(function () {
    // Multiple modals support
    $('.modal').on('hidden.bs.modal', function () {
        $(this).removeClass('fv-modal-stack')
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') - 1)
    })
    $('.modal').on('shown.bs.modal', function () {
        if (typeof ($('body').data('fv_open_modals')) == 'undefined') $('body').data('fv_open_modals', 0)
        if ($(this).hasClass('fv-modal-stack')) return
        $(this).addClass('fv-modal-stack')
        $('body').data('fv_open_modals', $('body').data('fv_open_modals') + 1)
        $(this).css('z-index', 1040 + (10 * $('body').data('fv_open_modals')))
        $('.modal-backdrop').not('.fv-modal-stack').css('z-index', 1039 + (10 * $('body').data('fv_open_modals')))
        $('.modal-backdrop').not('fv-modal-stack').addClass('fv-modal-stack')
    })
    $.fn.modal.Constructor.prototype.enforceFocus = function () {
        $(document).off('focusin.bs.modal').on('focusin.bs.modal', $.proxy(function (e) {
            if (this.$element[0] !== e.target && !this.$element.has(e.target).length
                && !$(e.target).closest('.cke_dialog, .cke').length) {
                this.$element.trigger('focus')
            }
        }, this))
    }
    $(document).on('hidden.bs.modal', '.modal', function () {
        $('.modal:visible').length && $(document.body).addClass('modal-open')
    })
    $('#modal').on('hidden.bs.modal', function () { dismiss() })
    $("#sendTestEmailModal").on("hidden.bs.modal", function () { dismissSendTestEmailModal() })
    $("#interface_type").on('change', function () { toggleInterface() })
    $("#insertHttpSample").on('click', function () { insertHttpSample() })
    $("#addCustomHeader").on('click', function () {
        var headerKey = $("#headerKey").val()
        var headerValue = $("#headerValue").val()
        if (!headerKey || !headerValue) return false
        addCustomHeader(headerKey, headerValue)
        $("#headerKey").val('').focus()
        $("#headerValue").val('')
        return false
    })
    $("#headersTable").on("click", "span>i.fa-trash-o", function () {
        headers.DataTable().row($(this).parents('tr')).remove().draw()
    })
    load()
})
