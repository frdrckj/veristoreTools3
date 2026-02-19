function loading(spinnerId, status) {
    var spinner = document.getElementById(spinnerId);
    if (spinner) { spinner.style.display = status ? 'block' : 'none'; }
}

function confirmation(text, spinnerId, formId) {
    Swal.fire({
        title: 'Konfirmasi', text: text, icon: 'warning',
        showCancelButton: true, confirmButtonText: 'Ya', cancelButtonText: 'Tidak'
    }).then((result) => {
        if (result.isConfirmed) {
            if (spinnerId) loading(spinnerId, true);
            if (formId) document.getElementById(formId).submit();
        }
    });
}

function confirmationEnglish(text, spinnerId, formId) {
    Swal.fire({
        title: 'Confirmation', text: text, icon: 'warning',
        showCancelButton: true, confirmButtonText: 'Yes', cancelButtonText: 'No'
    }).then((result) => {
        if (result.isConfirmed) {
            if (spinnerId) loading(spinnerId, true);
            if (formId) document.getElementById(formId).submit();
        }
    });
}

function search(spinnerId, formId) {
    if (spinnerId) loading(spinnerId, true);
    if (formId) document.getElementById(formId).submit();
}
